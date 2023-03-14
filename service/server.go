package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/bikappa/arduino-jobs-manager/service/graphql"
	"github.com/bikappa/arduino-jobs-manager/service/graphql/server"
	jobservice "github.com/bikappa/arduino-jobs-manager/service/jobservice"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type ServiceConfig struct {
	Port                     string
	Environment              string
	CompilationsK8SNamespace string
}

var serviceConfig ServiceConfig = ServiceConfig{
	Port:                     "8080",
	Environment:              "local",
	CompilationsK8SNamespace: "arduino-builder-api-jobs",
}

func main() {

	compilationService, err := createCompilationService()
	if err != nil {
		panic(err)
	}

	resolver, err := graphql.NewResolver(compilationService)
	if err != nil {
		panic(err)
	}

	gqlSrv := handler.New(server.NewExecutableSchema(server.Config{Resolvers: resolver}))

	gqlSrv.AddTransport(transport.Options{})
	gqlSrv.AddTransport(transport.GET{})
	gqlSrv.AddTransport(transport.POST{})

	gqlSrv.Use(extension.Introspection{})
	gqlSrv.AddTransport(transport.Websocket{
		InitFunc: server.IdentityWsInitFunc,
	})

	artefactsServer := NewArtefactServer(compilationService)

	http.Handle("/", playground.Handler("GraphQL playground", "/query"))
	http.Handle("/query", server.IdentityHttpMiddleware(gqlSrv))
	http.Handle("/artefacts", server.IdentityHttpMiddleware(artefactsServer))

	log.Printf("connect to http://localhost:%s/ for GraphQL playground", serviceConfig.Port)
	log.Fatal(http.ListenAndServe(":"+serviceConfig.Port, nil))
}

func NewArtefactServer(svc jobservice.CompilationService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("compilationId") == "" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "missing compilation id")
			return
		}
		compilationId := r.URL.Query().Get("compilationId")
		userId := server.ContextIdentity(r.Context())

		fileName, tarReader, err := svc.GetTarball(r.Context(), userId, compilationId)
		if err != nil {
			// TODO: unify error reporting and return coherent codes
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/tar")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
		w.WriteHeader(http.StatusOK)
		io.Copy(w, tarReader)
	})
}

func createCompilationService() (*jobservice.K8SCompilationService, error) {

	var config *rest.Config
	var err error
	if os.Getenv("IN_K8S_CLUSTER") == "true" {
		config, err = rest.InClusterConfig()
		if err != nil {
			panic(err.Error())
		}
	} else {
		var kubeconfig *string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
		} else {
			kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
		}
		flag.Parse()

		config, err = clientcmd.BuildConfigFromFlags("", *kubeconfig)
		if err != nil {
			panic(err)
		}
	}

	return jobservice.NewK8SCompilationService(jobservice.K8SJobServiceConfiguration{
		K8SRestClient: *config,
		ServiceMetadata: jobservice.ServiceMetadata{
			ID:      "builderApi",
			Version: "v2.0.0",
		},

		K8SNamespace: serviceConfig.CompilationsK8SNamespace,
		Environment: jobservice.Environment{
			ID: serviceConfig.Environment,
		},
		Jobs: jobservice.K8SJobServiceJobConfiguration{
			TTLSecondsAfterFinished: 60,
			ImageTag:                "arduino/cli",
			BuildVolume: jobservice.JobVolume{
				ClaimName: "compilation-output-volume-claim",
				SubPath:   "",
			},
			LibraryVolume: jobservice.JobVolume{
				ClaimName: "library-volume-claim",
				SubPath:   "",
			},
		},
		CompilationsOutputBaseDir: "/var/run/compilations-output",
	})

}
