// Copyright © 2018 AMIS Technologies
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package indexer

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/getamis/sirius/log"
	"github.com/maichain/eth-indexer/cmd/flags"
	"github.com/maichain/eth-indexer/service/indexer"
	"github.com/maichain/eth-indexer/store"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	configFile string

	// flags for ethereum service
	ethProtocol string
	ethHost     string
	ethPort     int

	// flags for database
	dbDriver   string
	dbHost     string
	dbPort     int
	dbName     string
	dbUser     string
	dbPassword string

	// flags for syncing
	targetBlock      int64
	syncMissingBlock bool
)

// RootCmd represents the base command when called without any subcommands
var ServerCmd = &cobra.Command{
	Use:   "indexer",
	Short: "blockchain data indexer",
	Long:  `blockchain data indexer`,
	RunE: func(cmd *cobra.Command, args []string) error {
		vp := viper.New()
		vp.BindPFlags(cmd.Flags())
		vp.AutomaticEnv() // read in environment variables that match
		if configFile := vp.GetString(flags.ConfigFileFlag); configFile != "" {
			if err := loadConfigUsingViper(vp, configFile); err != nil {
				log.Error("Failed to load config file", "err", err)
				return err
			}
			loadFlagToVar(vp)
		}

		// eth-client
		ethClient, err := NewEthConn(fmt.Sprintf("%s://%s:%d", ethProtocol, ethHost, ethPort))
		if err != nil {
			log.Error("Failed to new a eth client", "err", err)
			return err
		}
		defer ethClient.Close()

		// database
		db, err := NewDatabase()
		if err != nil {
			log.Error("Failed to connect to db", "err", err)
			return err
		}
		defer db.Close()

		sigs := make(chan os.Signal, 1)
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
			defer signal.Stop(sigs)

			log.Debug("Shutting down", "signal", <-sigs)
			cancel()
		}()

		indexer := indexer.New(ethClient, store.NewManager(db))
		if targetBlock > 0 {
			err = indexer.SyncToTarget(ctx, targetBlock)
		} else {
			ch := make(chan *types.Header)
			err = indexer.Listen(ctx, ch, syncMissingBlock)
		}

		// Ignore if listener is stopped by signal
		if err == context.Canceled {
			return nil
		}
		cancel()
		return err
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := ServerCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

func init() {
	// eth-client flags
	ServerCmd.Flags().StringVar(&ethProtocol, flags.EthProtocolFlag, "ws", "The eth-client protocol")
	ServerCmd.Flags().StringVar(&ethHost, flags.EthHostFlag, "127.0.0.1", "The eth-client host")
	ServerCmd.Flags().IntVar(&ethPort, flags.EthPortFlag, 8546, "The eth-client port")

	// Database flags
	ServerCmd.Flags().StringVar(&dbDriver, flags.DbDriverFlag, "mysql", "The database driver")
	ServerCmd.Flags().StringVar(&dbHost, flags.DbHostFlag, "", "The database host")
	ServerCmd.Flags().IntVar(&dbPort, flags.DbPortFlag, 3306, "The database port")
	ServerCmd.Flags().StringVar(&dbName, flags.DbNameFlag, "eth-db", "The database name")
	ServerCmd.Flags().StringVar(&dbUser, flags.DbUserFlag, "root", "The database username to login")
	ServerCmd.Flags().StringVar(&dbPassword, flags.DbPasswordFlag, "my-secret-pw", "The database password to login")

	// Syncing related flags
	ServerCmd.Flags().Int64Var(&targetBlock, flags.SyncTargetBlockFlag, 0, "The block number to sync to initially")
	ServerCmd.Flags().BoolVar(&syncMissingBlock, flags.SyncGetMissingBlocksFlag, false, "Whether to get state for the current block")
}

func loadConfigUsingViper(vp *viper.Viper, filename string) error {
	f, err := os.Open(strings.TrimSpace(filename))
	if err != nil {
		return err
	}
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}

	vp.SetConfigType("yaml")
	vp.ReadConfig(bytes.NewBuffer(b))

	return nil
}

func loadFlagToVar(vp *viper.Viper) {
	// flags for ethereum service
	ethProtocol = vp.GetString(flags.EthProtocolFlag)
	ethHost = vp.GetString(flags.EthHostFlag)
	ethPort = vp.GetInt(flags.EthPortFlag)

	// flags for database
	dbDriver = vp.GetString(flags.DbDriverFlag)
	dbHost = vp.GetString(flags.DbHostFlag)
	dbPort = vp.GetInt(flags.DbPortFlag)
	dbName = vp.GetString(flags.DbNameFlag)
	dbUser = vp.GetString(flags.DbUserFlag)
	dbPassword = vp.GetString(flags.DbPasswordFlag)

	// flags for syncing
	targetBlock = vp.GetInt64(flags.SyncTargetBlockFlag)
	syncMissingBlock = vp.GetBool(flags.SyncGetMissingBlocksFlag)
}
