// cmd/server/main.go
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/omnes/eip7702/eip7702"
)

func main() {
	godotenv.Load()

	rpcURL := os.Getenv("RPC_URL")
	if rpcURL == "" {
		log.Fatal("RPC_URL não definido")
	}

	rpc, err := eip7702.NewEthRPCClient(rpcURL)
	if err != nil {
		log.Fatal(err)
	}

	chainID, err := rpc.ChainID()
	if err != nil {
		log.Fatalf("Failed to get chain ID: %v", err)
	}

	if chainID == nil {
		log.Fatal("Chain ID is nil")
	}

	svc := &eip7702.DelegationService{ChainID: chainID, RPC: rpc}
	if svc.RPC == nil {
		log.Fatal("RPC client is nil in service")
	}

	h := eip7702.NewDelegationHandlers(svc)

	log.Printf("EIP-7702 API online – chainID %v", chainID)
	log.Fatal(http.ListenAndServe(":8080", h.Routes()))
}
