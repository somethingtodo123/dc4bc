package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/depools/dc4bc/client"
	"github.com/depools/dc4bc/qr"
	"github.com/depools/dc4bc/storage"

	"github.com/spf13/cobra"
)

const (
	flagUserName     = "username"
	flagListenAddr   = "listen_addr"
	flagStateDBDSN   = "state_dbdsn"
	flagStorageDBDSN = "storage_dbdsn"
	flagStorageTopic = "storage_topic"
	flagStoreDBDSN   = "key_store_dbdsn"
	flagFramesDelay  = "frames_delay"
	flagChunkSize    = "chunk_size"
	flagConfigPath   = "config_path"
)

func init() {
	rootCmd.PersistentFlags().String(flagUserName, "testUser", "Username")
	rootCmd.PersistentFlags().String(flagListenAddr, "localhost:8080", "Listen Address")
	rootCmd.PersistentFlags().String(flagStateDBDSN, "./dc4bc_client_state", "State DBDSN")
	rootCmd.PersistentFlags().String(flagStorageDBDSN, "./dc4bc_file_storage", "Storage DBDSN")
	rootCmd.PersistentFlags().String(flagStorageTopic, "messages", "Storage Topic (Kafka)")
	rootCmd.PersistentFlags().String(flagStoreDBDSN, "./dc4bc_key_store", "Key Store DBDSN")
	rootCmd.PersistentFlags().Int(flagFramesDelay, 10, "Delay times between frames in 100ths of a second")
	rootCmd.PersistentFlags().Int(flagChunkSize, 256, "QR-code's chunk size")
	rootCmd.PersistentFlags().String(flagConfigPath, "", "Path to a config file")
}

type config struct {
	Username      string `json:"username"`
	ListenAddress string `json:"listen_address"`
	StateDBDSN    string `json:"state_dbdsn"`
	StorageDBDSN  string `json:"storage_dbdsn"`
	StorageTopic  string `json:"storage_topic"`
	KeyStoreDBDSN string `json:"keystore_dbdsn"`
	FPS           int    `json:"frames_delay"`
	ChunkSize     int    `json:"chunk_size"`
}

func readConfig(path string) (config, error) {
	var cfg config
	configBz, err := ioutil.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file: %w", err)
	}
	if err = json.Unmarshal(configBz, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return cfg, nil
}

func loadConfig(cmd *cobra.Command) (*config, error) {
	var cfg config
	cfgPath, err := cmd.Flags().GetString(flagConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration: %v", err)
	}
	if cfgPath != "" {
		cfg, err = readConfig(cfgPath)
		if err != nil {
			return nil, err
		}
	} else {
		cfg.Username, err = cmd.Flags().GetString(flagUserName)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}
		cfg.KeyStoreDBDSN, err = cmd.Flags().GetString(flagStoreDBDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}

		cfg.ListenAddress, err = cmd.Flags().GetString(flagListenAddr)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}

		cfg.StateDBDSN, err = cmd.Flags().GetString(flagStateDBDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}

		cfg.FPS, err = cmd.Flags().GetInt(flagFramesDelay)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}

		cfg.ChunkSize, err = cmd.Flags().GetInt(flagChunkSize)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}

		cfg.StorageDBDSN, err = cmd.Flags().GetString(flagStorageDBDSN)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}

		cfg.StorageTopic, err = cmd.Flags().GetString(flagStorageTopic)
		if err != nil {
			return nil, fmt.Errorf("failed to read configuration: %v", err)
		}
	}
	return &cfg, nil
}

func genKeyPairCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "gen_keys",
		Short: "generates a keypair to sign and verify messages",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			keyPair := client.NewKeyPair()
			keyStore, err := client.NewLevelDBKeyStore(cfg.Username, cfg.KeyStoreDBDSN)
			if err != nil {
				return fmt.Errorf("failed to init key store: %w", err)
			}
			if err = keyStore.PutKeys(cfg.Username, keyPair); err != nil {
				return fmt.Errorf("failed to save keypair: %w", err)
			}
			fmt.Printf("keypair generated for user %s and saved to %s\n", cfg.Username, cfg.KeyStoreDBDSN)
			return nil
		},
	}
}

func startClientCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "starts dc4bc client",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(cmd)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)

			state, err := client.NewLevelDBState(cfg.StateDBDSN)
			if err != nil {
				log.Fatalf("Failed to init state client: %v", err)
			}

			stg, err := storage.NewKafkaStorage(ctx, cfg.StorageDBDSN, cfg.StorageTopic)
			if err != nil {
				log.Fatalf("Failed to init storage client: %v", err)
			}

			keyStore, err := client.NewLevelDBKeyStore(cfg.Username, cfg.KeyStoreDBDSN)
			if err != nil {
				log.Fatalf("Failed to init key store: %v", err)
			}

			processor := qr.NewCameraProcessor()
			processor.SetDelay(cfg.FPS)
			processor.SetChunkSize(cfg.ChunkSize)

			cli, err := client.NewClient(ctx, cfg.Username, state, stg, keyStore, processor)
			if err != nil {
				log.Fatalf("Failed to init client: %v", err)
			}

			sigs := make(chan os.Signal, 1)
			signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigs

				log.Println("Received signal, stopping client...")
				cancel()

				log.Println("BaseClient stopped, exiting")
				os.Exit(0)
			}()

			go func() {
				if err := cli.StartHTTPServer(cfg.ListenAddress); err != nil {
					log.Fatalf("HTTP server error: %v", err)
				}
			}()
			cli.GetLogger().Log("starting to poll messages from append-only log...")
			if err = cli.Poll(); err != nil {
				log.Fatalf("error while handling operations: %v", err)
			}
			cli.GetLogger().Log("polling is stopped")
			return nil
		},
	}
}

var rootCmd = &cobra.Command{
	Use:   "dc4bc_d",
	Short: "dc4bc client daemon implementation",
}

func main() {
	rootCmd.AddCommand(
		startClientCommand(),
		genKeyPairCommand(),
	)
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Failed to execute root command: %v", err)
	}
}
