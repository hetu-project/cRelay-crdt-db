package orbitdb

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	orbitdb "berty.tech/go-orbit-db"
	"berty.tech/go-orbit-db/accesscontroller"
	"berty.tech/go-orbit-db/iface"

	// "github.com/ipfs/go-cid"
	ipfsCore "github.com/ipfs/kubo/core"
	"github.com/ipfs/kubo/core/coreapi"
)

var (
	ipfsNode    *ipfsCore.IpfsNode
	orbitDB     iface.OrbitDB
	documentDB  iface.DocumentStore
	initOnce    sync.Once
	initialized bool
	dbName      string
	orbitDBDir  string
)

// Init initializes the database connection
func Init(name string, orbitdir string) error {
	dbName = name
	orbitDBDir = orbitdir
	var initErr error

	initOnce.Do(func() {

		if err := os.MkdirAll(orbitDBDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create directory %s: %w", orbitDBDir, err)
			return
		}

		// Initialize IPFS node
		ctx := context.Background()
		ipfsNode, err := ipfsCore.NewNode(ctx, &ipfsCore.BuildCfg{
			Online: true,
			// NilRepo: false,
			ExtraOpts: map[string]bool{
				"pubsub": true,
				"mplex":  true,
			},
		})
		if err != nil {
			initErr = fmt.Errorf("failed to initialize IPFS node: %w", err)
			return
		}

		// errs := ipfsNode.DHT.Provide(ctx, cid.Undef, true)
		// if errs != nil {
		// 	log.Printf("DHT advertisement failed: %v", errs)
		// }
		// Relay service code
		peerID := ipfsNode.Identity.String()
		addrs := ipfsNode.PeerHost.Addrs()
		log.Printf("Relay IPFS node information:")
		log.Printf("Peer ID: %s", peerID)
		for _, addr := range addrs {
			log.Printf("Multiaddr: %s/p2p/%s", addr.String(), peerID)
		}

		// Get IPFS API
		api, err := coreapi.NewCoreAPI(ipfsNode)
		if err != nil {
			initErr = fmt.Errorf("failed to create IPFS API: %w", err)
			return
		}

		// Create OrbitDB instance
		orbitDB, err = orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
			Directory: &orbitDBDir,
		})
		if err != nil {
			initErr = fmt.Errorf("failed to create OrbitDB instance: %w", err)
			return
		}

		// Create document database
		create := true
		dbOptions := &orbitdb.CreateDBOptions{
			AccessController: &accesscontroller.CreateAccessControllerOptions{
				Type: "ipfs",
				Access: map[string][]string{
					"write": {"*"},
					"read":  {"*"},
				},
			},
			Directory: &orbitDBDir,
			Create:    &create,
		}

		db, err := orbitDB.Docs(ctx, dbName, dbOptions)
		if err != nil {
			initErr = fmt.Errorf("failed to create document database: %w", err)
			return
		}
		documentDB = db

		initialized = true
		addr := documentDB.Address().String()
		log.Printf("Document database address: %s", addr)
		log.Println("Database initialization successful")
	})

	return initErr
}

// GetStore gets the initialized OrbitDB store instance
func GetStore() (iface.DocumentStore, error) {
	if !initialized || documentDB == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	return documentDB, nil
}

// Close closes the database connection
func Close() error {
	if documentDB != nil {
		documentDB.Close()
	}

	if orbitDB != nil {
		orbitDB.Close()
	}

	if ipfsNode != nil {
		if err := ipfsNode.Close(); err != nil {
			return fmt.Errorf("failed to close IPFS node: %w", err)
		}
	}

	initialized = false
	return nil
}
