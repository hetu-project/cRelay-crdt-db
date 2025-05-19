#!/bin/bash
set -e

# Configuration section
DEFAULT_DATA_DIR="$HOME/data"
DEFAULT_API_DATA_DIR="$HOME/api-data"
IPFS_DIR="${1:-$DEFAULT_DATA_DIR/ipfs}"
NAK_LOG="$DEFAULT_DATA_DIR/nak.log"
ORBITABI_LOG="$DEFAULT_DATA_DIR/orbitabi.log"
TIMEOUT=60  # Timeout for waiting log output (seconds)

# Function to clean previous data
clean_previous_data() {
    echo "🧹 Cleaning previous data..."
    
    # Delete log files
    rm -f "$NAK_LOG" "$ORBITABI_LOG" 2>/dev/null
    
    # Delete database directories
    rm -rf \
        "$DEFAULT_DATA_DIR/orbitdb" \
        "$DEFAULT_API_DATA_DIR/orbitdb" \
        "$IPFS_DIR" 2>/dev/null
    
    # Recreate base directories
    mkdir -p "$DEFAULT_DATA_DIR" "$DEFAULT_API_DATA_DIR"
}

# 1. Check and install IPFS
install_ipfs() {
    if ! command -v ipfs &> /dev/null; then
        echo "❌ IPFS not installed, starting installation..."
        
        # Install dependencies
        sudo apt-get update
        sudo apt-get install -y wget
        
        # Download specific version
        wget https://dist.ipfs.tech/kubo/v0.34.1/kubo_v0.34.1_linux-amd64.tar.gz
        tar -xvzf kubo_v0.34.1_linux-amd64.tar.gz
        sudo ./kubo/install.sh
        rm -rf kubo*
        
        # Verify installation
        if ! command -v ipfs &> /dev/null; then
            echo "⚠️ IPFS installation failed, please install manually"
            exit 1
        fi
        echo "✅ IPFS installation successful"
    fi
}

# 2. Check and install nak
install_nak() {
    if ! command -v nak &> /dev/null; then
        echo "❌ nak not installed, starting installation..."
        
        # Clone repository
        git clone https://github.com/hetu-project/cRelay-nak.git
        cd cRelay-nak
        
        # Build and install
        go install ./...
        
        # Verify installation
        if ! command -v nak &> /dev/null; then
            echo "⚠️ nak installation failed, please install manually"
            exit 1
        fi
        echo "✅ nak installation successful"
        cd ..
    fi
}

# 3. Clean IPFS lock files
clean_ipfs_locks() {
    LOCK_FILES=("$IPFS_DIR/repo.lock" "$IPFS_DIR/api")
    for lock in "${LOCK_FILES[@]}"; do
        if [ -f "$lock" ]; then
            echo "🔓 Cleaning lock file: $lock"
            rm -f "$lock"
        fi
    done
}

# 4. Initialize IPFS repository
init_ipfs() {
    if [ ! -d "$IPFS_DIR/config" ]; then
        echo "🔄 Initializing IPFS repository ($IPFS_DIR)..."
        # Note: Set IPFS_PATH before initialization
        export IPFS_PATH="$IPFS_DIR"
        ipfs init --profile server -e
    else
        # Still need to set environment variable for existing repository
        export IPFS_PATH="$IPFS_DIR"
    fi
}

# 5. Start nak service and extract key information
start_nak() {
    echo "🚀 Starting nak service..."
    export IPFS_PATH="$IPFS_DIR"
        nohup nak serve \
        --hostname 0.0.0.0 \
        --orbitdb-dir "$DEFAULT_DATA_DIR/orbitdb" \
        > "$NAK_LOG" 2>&1 &
    
    # Wait and extract key information
    echo "⏳ Waiting for nak service initialization..."
    local start_time=$(date +%s)
    
    while true; do
        # Get first Multiaddr
        MULTIADDR=$(grep -m1 'Multiaddr: ' "$NAK_LOG" | awk '{print $2}')
        DB_ADDRESS=$(grep -m1 'Document database address: ' "$NAK_LOG" | awk '{print $2}')
        
        # Check timeout
        if [ $(($(date +%s) - start_time)) -gt $TIMEOUT ]; then
            echo "⏰ Wait timeout, please check logs: $NAK_LOG"
            exit 1
        fi
        
        # Verify obtained information
        if [[ -n "$MULTIADDR" && -n "$DB_ADDRESS" ]]; then
            echo "✅ Key parameters obtained:"
            echo "   Multiaddr: $MULTIADDR"
            echo "   Database address: $DB_ADDRESS"
            break
        fi
        sleep 1
    done
}

# 6. Start orbitabi service
start_orbitabi() {
    echo "🚀 Starting cRelay-crdt-db service..."
        nohup orbitabi \
        -db "$DB_ADDRESS" \
        -Multiaddr "$MULTIADDR" \
        -orbitdb-dir "$DEFAULT_API_DATA_DIR/orbitdb" \
        > "$ORBITABI_LOG" 2>&1 &
    
    # Verify startup
    sleep 3
    if ! pgrep -f "cRelay-crdt-db" > /dev/null; then
        echo "⚠️ cRelay-crdt-db startup failed, please check logs: $ORBITABI_LOG"
        exit 1
    fi
    
    echo "✅ Service startup completed"
    echo "========================"
    echo "nak logs: $NAK_LOG"
    echo "cRelay-crdt-db logs: $ORBITABI_LOG"
    echo "IPFS directory: $IPFS_DIR"
}

# Main execution flow
main() {
    clean_previous_data
    install_ipfs
    install_nak
    clean_ipfs_locks
    init_ipfs
    start_nak
    start_orbitabi
}

# Execute main function
main
