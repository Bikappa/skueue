package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
)

func TestXxx(t *testing.T) {
	apiClient, startws := NewClient("localhost:8081", "bikappino")
	go startws(context.Background())
	startedJob, _, err := apiClient.StartCompilation(context.Background(), "arduino:avr:uno", Sketch{
		Name: "Blink",
		Ino: File{
			Name: "Blink.ino",
			Data: `
				void setup() {}
				void loop() {}
			`,
		},
	}, nil, nil)
	if err != nil {
		panic(err)
	}
	gotJob, _, err := apiClient.GetCompilation(context.Background(), *startedJob.StartCompilation.ID)
	if err != nil {
		panic(err)
	}
	fmt.Println("compilation id:", *gotJob.Compilation.ID)

	updatesChannel, cancelUpdates := apiClient.CompilationUpdateSubscription(context.Background(), *gotJob.Compilation.ID)
	defer cancelUpdates()

	logsChannel, cancelLogs := apiClient.CompilationLogSubscription(context.Background(), *gotJob.Compilation.ID)
	defer cancelLogs()

	wg := sync.WaitGroup{}
	wg.Add(2)

	go func() {
		defer wg.Done()
		for e := range updatesChannel {
			if e.Error != nil {
				fmt.Println("update error", e.Error.Error())
			}
			fmt.Println("update", e.Data.CompilationUpdate.State)
		}
	}()
	go func() {
		defer wg.Done()
		for e := range logsChannel {
			if e.Error != nil {
				fmt.Println("log error", e.Error.Error())
			}
			fmt.Println("log", e.Data.CompilationLog.Text)
		}
	}()

	wg.Wait()
	reader, err := apiClient.GetCompilationTarball(context.Background(), *gotJob.Compilation.ID)
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
