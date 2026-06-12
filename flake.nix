# flake.nix
{
  description = "Isolated Antigravity CLI environment with embedded API configurations";
  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      # Define your architecture (change to "aarch64-linux" if on an ARM laptop)
      system = "x86_64-linux"; 
      pkgs = import nixpkgs { 
        inherit system;
        config.allowUnfree = true; 
      };

      # Creates a secure Linux FHS environment matching standard distributions (Ubuntu/Debian)
      # This allows Google's installation scripts and downloaded binaries to run flawlessly on NixOS.
      antigravity-env = pkgs.buildFHSEnv {
        name = "agy-env";
        targetPkgs = pkgs: with pkgs; [
          curl
          git
          gnumake
          glibc
          libxcrypt
          coreutils
        ];

      # Automatically injects your API configurations whenever the sandbox fires up
      profile = ''
          # Force the installer and bin paths to live safely inside your local home folder
          export PATH="$HOME/.local/bin:$PATH"
        '';
      };

    in {
      # Allows running 'nix develop' to enter the isolated container
      devShells.${system}.default = pkgs.mkShell {
        nativeBuildInputs = [ antigravity-env ];
        # Packages available natively in your shell path (no FHS containment)
        packages = with pkgs; [
            # --- Antigravity Environment Runner ---
            antigravity-env
            # --- Browser Testing Toolchain ---
            google-chrome
            chromedriver
            # --- Go Development Stack ---
            go
            gopls         # Go Language Server
            gotools       # godoc, goimports, etc.
            podman-compose

            # --- Ruby on Rails Stack ---
            ruby_3_3      # Matches stable Rails deployment requirements
            bundler
            rufo          # Ruby formatter
            
            # Native extensions dependencies commonly required by Ruby gems (like nokogiri, pg, sqlite3)
            pkg-config
            libxml2
            libxslt
            libyaml
            zlib
            sqlite
            postgresql    # Includes libpq for the 'pg' gem
            openssl
            redis
            rabbitmq-server
          ];


          shellHook = ''
            echo "========================================================="
            echo "🚀 Antigravity CLI FHS Sandbox Loaded!"
            echo "========================================================="

            # Tells podman-compose to strictly adhere to Docker naming compatibility constraints
            export PODMAN_COMPOSE_DOCKER_COMPOSE_COMPAT=true
            # Create an alias so 'docker-compose' commands translate cleanly
            alias docker-compose="podman compose"
            echo "🐳 Podman Compose Integration Active!"

            # Drop directly into the FHS user environment loop
            # exec agy-env
            # --- Go Environment Configuration ---
            export GOPATH="$PWD/.go"
            export GOBIN="$GOPATH/bin"
            export PATH="$GOBIN:$PATH"

            # --- Ruby / Bundler Configuration ---
            # Keeps your gem paths local to the directory so they don't break system pathways
            export GEM_HOME="$PWD/.bundle/gems"
            export GEM_PATH="$GEM_HOME"
            export PATH="$GEM_HOME/bin:$PATH"

            # --- Rails Native Extensions Helper Flags ---
            # Tells bundler exactly where Nix keeps header libraries during 'bundle install'
            export BUNDLE_BUILD__NOKOGIRI="--use-system-libraries"

            # ==========================================
            # 2. Ephemeral Database Service Setup
            # ==========================================
            export DEV_DATA_DIR="$PWD/.nix_data"
            export PGDATA="$DEV_DATA_DIR/postgres"
            export REDIS_DIR="$DEV_DATA_DIR/redis"
            export PGHOST="localhost"
            export PGPORT="5432"

            # Ensure data directories exist
            mkdir -p "$PGDATA" "$REDIS_DIR"

            # --- Initialize Postgres Database System ---
            if [ ! -d "$PGDATA/base" ]; then
              echo "⚙️ Initializing a fresh local PostgreSQL cluster..."
              initdb --auth=trust -U postgres "$PGDATA" > /dev/null
            fi

            # --- Start Services ---
            echo "🚀 Starting background services..."
            
            # Start PostgreSQL (logs routed safely out of standard stdout stdout/stderr output)
            pg_ctl -o "-p $PGPORT -k $PGDATA" -l "$PGDATA/server.log" start > /dev/null 2>&1
            
            # Start Redis
            redis-server --port 6379 --dir "$REDIS_DIR" --daemonize yes > /dev/null

            # --- RabbitMQ Non-Root User Config ---
            export RABBITMQ_BASE="$DEV_DATA_DIR/rabbitmq"
            export RABBITMQ_MNESIA_BASE="$RABBITMQ_BASE/mnesia"
            export RABBITMQ_LOG_BASE="$RABBITMQ_BASE/logs"
            export RABBITMQ_PID_FILE="$RABBITMQ_BASE/rabbitmq.pid"
            export RABBITMQ_SCHEMA_DIR="$RABBITMQ_BASE/schema"
            export RABBITMQ_GENERATED_CONFIG_DIR="$RABBITMQ_BASE/config"
            
            # Force RabbitMQ to run locally without hitting system-wide configs
            export RABBITMQ_NODENAME="rabbit@localhost"
            export RABBITMQ_NODE_PORT="5672"
            
            # Keep the erlang cookie local to the workspace
            export HOME="$PWD" 

            mkdir -p "$RABBITMQ_MNESIA_BASE" "$RABBITMQ_LOG_BASE" "$RABBITMQ_SCHEMA_DIR" "$RABBITMQ_GENERATED_CONFIG_DIR"
            # Start RabbitMQ detached
            rabbitmq-server -detached > /dev/null 2>&1
            
            # ==========================================
            # 3. Automatic Service Teardown Function
            # ==========================================
            trap_exit() {
              echo -e "\n🛑 Tearing down development background services..."
              pg_ctl stop -D "$PGDATA" -m fast > /dev/null 2>&1
              redis-cli -p 6379 shutdown > /dev/null 2>&1
              rabbitmqctl stop > /dev/null 2>&1
            }
            # Attach the shutdown code to the shell exit handler
            trap trap_exit EXIT

            # ==========================================
            # 4. Welcome Screen & Verification
            # ==========================================
            echo "====================================================================="
            echo "🌟 Isolated Full-Stack Workspace Ready!"
            echo "     • Go:        $(go version | awk '{print $3}')"
            echo "     • Ruby:      $(ruby --version | awk '{print $2}')"
            echo "     • Postgres:  Running on port $PGPORT (User: 'postgres', No Password)"
            echo "     • Redis:     Running on port 6379"
            echo "     • RabbitMQ:  Running on amqp://localhost:5672"
            echo "====================================================================="
            echo "👉 Run 'agy-env' to drop into the Antigravity TUI agent container."
            echo "====================================================================="

            '';
        };
      };
    }
