package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/bikappa/arduino-jobs-manager/client"
)

func main() {
	apiClient, startws := client.NewClient("localhost:3001", "bikappino")
	go startws(context.Background())
	verbose := true
	startedJob, _, err := apiClient.StartCompilation(context.Background(), client.CompilationParameters{
		Fqbn: "arduino:avr:uno",
		Sketch: client.Sketch{
			Name: "Blink",
			Ino: client.File{
				Name: "Blink.ino",
				Data: `
					void setup() {}
					void loop() {}
				`,
			},
		},
		Verbose: &verbose,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("Started compilation")

	fetchedJob, _, err := apiClient.GetCompilation(context.Background(), startedJob.StartCompilation.ID)
	if err != nil {
		panic(err)
	}
	fmt.Println("Compilation id:", fetchedJob.Compilation.ID)

	updatesChannel, cancelUpdates := apiClient.CompilationUpdateSubscription(context.Background(), fetchedJob.Compilation.ID)
	defer cancelUpdates()

	logsChannel, cancelLogs := apiClient.CompilationLogSubscription(context.Background(), fetchedJob.Compilation.ID)
	defer cancelLogs()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for e := range updatesChannel {
			if e.Error != nil {
				fmt.Println("Updates stream error:", e.Error.Error())
			}
			fmt.Println("Updates stream:", e.Data.CompilationUpdate.CompilationJob.Status)
		}
	}()
	go func() {
		defer wg.Done()
		for e := range logsChannel {
			if e.Error != nil {
				fmt.Println("Logs stream error:", e.Error.Error())
			}
			fmt.Println("Logs stream:", e.Data.CompilationLog.Text)
		}
	}()

	wg.Wait()
	reader, err := apiClient.GetCompilationTarball(context.Background(), fetchedJob.Compilation.ID)
	if err != nil {
		panic(err)
	}
	file, err := os.Create("Blink.tar")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	io.Copy(file, reader)
}
