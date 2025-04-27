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
	ma "github.com/multiformats/go-multiaddr"

	// Import IPFS data storage drivers
	_ "github.com/ipfs/go-ds-badger"
	_ "github.com/ipfs/go-ds-flatfs"
	_ "github.com/ipfs/go-ds-leveldb"
	_ "github.com/ipfs/go-ds-measure"
	// "github.com/ipfs/kubo/core/node/libp2p"
)

var (
	dbAddress  = flag.String("db", "", "OrbitDB address to connect to")
	dataDir    = flag.String("data", "~/data", "Data directory path")
	listenAddr = flag.String("listen", "/ip4/0.0.0.0/tcp/4001", "Libp2p listen address")
	ipfssAPI   = flag.String("ipfs", "localhost:5001", "IPFS API endpoint")
	port       = flag.String("port", "8080", "API服务端口")
	StoreType  = "docstore" // eventlog|keyvalue|docstore
	Create     = true
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup data directories
	ipfsDir := filepath.Join(*dataDir, "ipfs")
	orbitDBDir := filepath.Join(*dataDir, "orbitdb")
	settingsDir := filepath.Join(*dataDir, "settings")

	// Ensure directories exist
	for _, dir := range []string{ipfsDir, orbitDBDir, settingsDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Get or generate peer identity
	// privKey, peerID, err := getOrCreatePeerID(settingsDir)
	// if err != nil {
	// 	log.Fatalf("Failed to get peer ID: %v", err)
	// }
	// log.Printf("Using Peer ID: %s", peerID.String())

	// Connect to local IPFS node
	// ipfsAPI, ipfsNode, err := InitIPFS(ipfsDir)
	// if err != nil {
	// 	log.Fatalf("Failed to initialize IPFS: %v", err)
	// }
	// defer ipfsNode.Close()
	node, _ := core.NewNode(ctx, &core.BuildCfg{
		Online: true, // 必须为 true，OrbitDB 需要网络功能
		// NilRepo: false, // 需要持久化存储
		ExtraOpts: map[string]bool{
			"pubsub": true, // OrbitDB 依赖 PubSub
			"mplex":  true, // 多路复用支持
		},
	})
	api, _ := coreapi.NewCoreAPI(node)
	// Initialize IPFS HTTP client
	// sh := shell.NewShell(*ipfssAPI)
	// if sh == nil {
	// 	log.Fatalf("Failed to initialize IPFS HTTP client")
	// }
	// 2. 转换为 coreapi 接口
	// api, err := coreapi.NewClient(sh)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// Setup libp2p host
	// host, err := setupLibp2p(ctx, privKey, *listenAddr)
	// if err != nil {
	// 	log.Fatalf("Failed to create libp2p host: %v", err)
	// }

	// Print peer addresses
	// addrs := host.Addrs()
	// var addrStrings []string
	// for _, addr := range addrs {
	// 	addrStrings = append(addrStrings, fmt.Sprintf("%s/p2p/%s", addr.String(), host.ID().String()))
	// }
	// log.Printf("Peer addresses: %s", strings.Join(addrStrings, ", "))

	// Create OrbitDB instance
	// orbit, err := orbitdb.NewOrbitDB(ctx, ipfsAPI, &orbitdb.NewOrbitDBOptions{
	// 	Directory: &orbitDBDir,
	// })
	// Explicitly try to enable pubsub
	// Create OrbitDB instance with explicit pubsub options
	orbit, err := orbitdb.NewOrbitDB(ctx, api, &orbitdb.NewOrbitDBOptions{
		Directory: &orbitDBDir,
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
			Directory: &orbitDBDir,
			Create:    &Create,
			StoreType: &StoreType,
		})
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer orbit.Close()
		db = dbInstance.(iface.DocumentStore)

		// 创建API路由器
		router := router.NewRouter(adapter.NewOrbitDBAdapter(db))

		// 启动HTTP服务器
		addr := fmt.Sprintf(":%s", *port)
		log.Printf("API服务启动在 %s", addr)
		if err := http.ListenAndServe(addr, router.Handler()); err != nil {
			log.Fatalf("HTTP服务器错误: %v", err)
		}

	} else {
		log.Fatal(`
                   错误：未指定数据库地址！
                   请先启动 relay 服务生成数据库地址，再通过 -db 参数运行此 API 服务。
                   示例命令：
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

// connectToExistingDB 连接 relay 创建的数据库
func connectToExistingDB(ctx context.Context, api coreiface.CoreAPI, dbAddress string) (iface.OrbitDB, iface.DocumentStore, error) {
	orbitInstance, err := orbitdb.NewOrbitDB(ctx, api, nil)
	if err != nil {
		return nil, nil, err
	}

	db, err := orbitInstance.Open(ctx, dbAddress, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("无法打开数据库：%w（请确认 relay 正在运行）", err)
	}

	return orbitInstance, db.(iface.DocumentStore), nil
}
