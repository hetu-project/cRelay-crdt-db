package main

import (
	"context"
	"net/http"

	// "encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	orbitdb "berty.tech/go-orbit-db"
	"berty.tech/go-orbit-db/iface"
	coreiface "github.com/ipfs/kubo/core/coreiface"

	// shell "github.com/ipfs/go-ipfs-api"
	core "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"

	// coreapi "github.com/ipfs/kubo/client/rpc"
	router "github.com/hetu-project/cRelay-crdt-db/internal/api"
	adapter "github.com/hetu-project/cRelay-crdt-db/orbitdb"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"

	// "github.com/multiformats/go-multiaddr"
	ma "github.com/multiformats/go-multiaddr"

	// Import IPFS data storage drivers
	_ "github.com/ipfs/go-ds-badger"
	_ "github.com/ipfs/go-ds-flatfs"
	_ "github.com/ipfs/go-ds-leveldb"
	_ "github.com/ipfs/go-ds-measure"
	// "github.com/ipfs/kubo/core/node/libp2p"
)

var (
	dbAddress      = flag.String("db", "", "OrbitDB address to connect to")
	relayMultiaddr = flag.String("Multiaddr", "", "relayMultiaddr")
	port           = flag.String("port", "8080", "API service port")
	orbitDBDir     = flag.String("orbitdb-dir", "", "OrbitDB data storage directory")
	// dbName        = flag.String("db-name", "", "Database name")
	StoreType = "docstore" // eventlog|keyvalue|docstore
	Create    = true
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *orbitDBDir == "" {
		home, _ := os.UserHomeDir()
		*orbitDBDir = filepath.Join(home, "api-data", "orbitdb")
	}
	log.Printf("API service OrbitDB database address: %s", *orbitDBDir)
	// Ensure directories exist
	if err := os.MkdirAll(*orbitDBDir, 0755); err != nil {
		log.Fatalf("Failed to create directory %s: %v", *orbitDBDir, err)
	}

	node, _ := core.NewNode(ctx, &core.BuildCfg{
		Online: true, // Must be true, OrbitDB requires network functionality
		// NilRepo: false, // Requires persistent storage
		ExtraOpts: map[string]bool{
			"pubsub": true, // OrbitDB depends on PubSub
			"mplex":  true, // Multiplexing support
		},
	})
	api, _ := coreapi.NewCoreAPI(node)

	orbit, err := orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
		Directory: orbitDBDir,
	})
	if err != nil {
		log.Fatalf("Failed to create OrbitDB instance: %v", err)
	}
	// Open or create database
	var db iface.DocumentStore
	if *dbAddress != "" {
		// Connect to existing database
		log.Printf("Connecting to database: %s", *dbAddress)
		dbInstance, err := orbit.Open(ctx, *dbAddress, &orbitdb.CreateDBOptions{
			Directory: orbitDBDir,
			Create:    &Create,
			StoreType: &StoreType,
		})
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		addr, _ := ma.NewMultiaddr(*relayMultiaddr)
		addrInfo, _ := peer.AddrInfoFromP2pAddr(addr)
		err = api.Swarm().Connect(ctx, *addrInfo)
		if err != nil {
			log.Printf("Failed to connect to Relay node: %v", err)
		} else {
			log.Printf("Successfully connected to Relay node")
		}
		defer orbit.Close()
		db = dbInstance.(iface.DocumentStore)
		newadd := db.Address().String()
		log.Printf("API database address: %s", newadd)
		// Create API router
		router := router.NewRouter(adapter.NewOrbitDBAdapter(db))

		// Start HTTP server
		addrs := fmt.Sprintf(":%s", *port)
		log.Printf("API service starting on %s", addrs)
		if err := http.ListenAndServe(addrs, router.Handler()); err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}

	} else {
		log.Fatal(`
                   Error: Database address not specified!
                   Please start the relay service first to generate a database address, then run this API service with the -db parameter.
                   Example command:
                   ./api-service -db /orbitdb/zdpuAm... -port 8080
		`)
		//log.Printf("Database created with address: %s", db.Address().String())
	}
}

// getOrCreatePeerID loads or creates a peer ID
func getOrCreatePeerID(settingsDir string) (crypto.PrivKey, peer.ID, error) {
	keyFile := filepath.Join(settingsDir, "peer.key")

	// Check if key file exists
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		// Generate new key
		priv, pub, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate key pair: %w", err)
		}

		// Get peer ID from public key
		pid, err := peer.IDFromPublicKey(pub)
		if err != nil {
			return nil, "", fmt.Errorf("failed to get peer ID: %w", err)
		}

		// Serialize private key
		keyBytes, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			return nil, "", fmt.Errorf("failed to marshal private key: %w", err)
		}

		// Save to file
		if err := ioutil.WriteFile(keyFile, keyBytes, 0600); err != nil {
			return nil, "", fmt.Errorf("failed to save key: %w", err)
		}

		log.Printf("Generated new peer ID: %s", pid.String())
		return priv, pid, nil
	}

	// Load existing key
	keyBytes, err := ioutil.ReadFile(keyFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read key file: %w", err)
	}

	// Unmarshal private key
	priv, err := crypto.UnmarshalPrivateKey(keyBytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal private key: %w", err)
	}

	// Get peer ID from public key
	pub := priv.GetPublic()
	pid, err := peer.IDFromPublicKey(pub)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get peer ID: %w", err)
	}

	log.Printf("Loaded existing peer ID: %s", pid.String())
	return priv, pid, nil
}

// setupLibp2p creates a libp2p host
func setupLibp2p(ctx context.Context, privKey crypto.PrivKey, listenAddr string) (host.Host, error) {
	addr, err := ma.NewMultiaddr(listenAddr)
	if err != nil {
		return nil, fmt.Errorf("invalid listen address: %w", err)
	}

	// Create libp2p host with only necessary options
	host, err := libp2p.New(
		libp2p.ListenAddrs(addr),
		libp2p.Identity(privKey),
		libp2p.EnableRelay(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}

	return host, nil
}

// connectToExistingDB connects to the database created by relay
func connectToExistingDB(ctx context.Context, api coreiface.CoreAPI, dbAddress string) (iface.OrbitDB, iface.DocumentStore, error) {
	orbitInstance, err := orbitdb.NewOrbitDB(ctx, api, nil)
	if err != nil {
		return nil, nil, err
	}

	db, err := orbitInstance.Open(ctx, dbAddress, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w (please ensure relay is running)", err)
	}

	return orbitInstance, db.(iface.DocumentStore), nil
}
