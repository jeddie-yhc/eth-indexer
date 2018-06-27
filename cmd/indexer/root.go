// Copyright 2018 The eth-indexer Authors
// This file is part of the eth-indexer library.
//
// The eth-indexer library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The eth-indexer library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the eth-indexer library. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/getamis/eth-indexer/cmd/flags"
	"github.com/getamis/eth-indexer/service/indexer"
	"github.com/getamis/eth-indexer/store"
	"github.com/getamis/sirius/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	cfgFileName = "config"
	cfgFileType = "yaml"
	cfgFilePath = "./configs"
)

var (
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
	targetBlock  int64
	fromBlock    int64
	onySubscribe bool

	// flags for profiling
	profiling  bool
	profilHost string
	profilPort int

	// flags for functions
	subscribeErc20token bool
)

// RootCmd represents the base command when called without any subcommands
var ServerCmd = &cobra.Command{
	Use:   "indexer",
	Short: "blockchain data indexer",
	Long:  `blockchain data indexer`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		indexer := indexer.New(ethClient, store.NewManager(db, onySubscribe))

		if subscribeErc20token {
			erc20Addresses, erc20BlockNumbers, err := LoadTokensFromConfig()
			if err != nil {
				log.Error("Fail to load ERC20Token List from Config File", "err", err)
				return err
			}
			log.Debug("erc20Addresses Successfully Loaded")

			if err := indexer.SubscribeErc20Tokens(ctx, erc20Addresses, erc20BlockNumbers); err != nil {
				log.Error("Fail to subscribe ERC20Tokens and write to database", "err", err)
				return err
			}
		}

		if profiling {
			// run `go tool pprof build/bin/service http://127.0.0.1:8000/debug/pprof/profile\?seconds\=60`
			// Start profiling
			go func() {
				url := fmt.Sprintf("%s:%d", profilHost, profilPort)
				log.Info("Starting profiling", "url", url)
				http.ListenAndServe(url, nil)
			}()
		}

		log.Info("Starting eth-indexer")
		if targetBlock > 0 {
			err = indexer.SyncToTarget(ctx, fromBlock, targetBlock)
		} else {
			ch := make(chan *types.Header)
			err = indexer.Listen(ctx, ch, fromBlock)
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
		log.Crit("ServerCmd Execution failed", "err", err)
	}
}

func init() {
	root, _ := os.Getwd()
	cfgFile := root + "/" + cfgFilePath + "/" + cfgFileName + "." + cfgFileType

	// Take cfgFile as the first priority to load and only enable flags when cfgFile does not exists.
	if _, err := os.Stat(cfgFile); os.IsNotExist(err) {
		log.Debug("The config file does not exist. Run ServerCmd.Flags().")
		// eth-client flags
		ServerCmd.Flags().StringVar(&ethProtocol, flags.EthProtocol, "ws", "The eth-client protocol")
		ServerCmd.Flags().StringVar(&ethHost, flags.EthHost, "127.0.0.1", "The eth-client host")
		ServerCmd.Flags().IntVar(&ethPort, flags.EthPort, 8546, "The eth-client port")

		// Database flags
		ServerCmd.Flags().StringVar(&dbDriver, flags.DbDriver, "mysql", "The database driver")
		ServerCmd.Flags().StringVar(&dbHost, flags.DbHost, "", "The database host")
		ServerCmd.Flags().IntVar(&dbPort, flags.DbPort, 3306, "The database port")
		ServerCmd.Flags().StringVar(&dbName, flags.DbName, "ethdb", "The database name")
		ServerCmd.Flags().StringVar(&dbUser, flags.DbUser, "root", "The database username to login")
		ServerCmd.Flags().StringVar(&dbPassword, flags.DbPassword, "my-secret-pw", "The database password to login")

		// Syncing related flags
		ServerCmd.Flags().Int64Var(&targetBlock, flags.SyncTargetBlock, 0, "The block number to sync to initially")
		ServerCmd.Flags().Int64Var(&fromBlock, flags.SyncFromBlock, 0, "The init block number to sync to initially")
		ServerCmd.Flags().BoolVar(&onySubscribe, flags.SyncOnlySubscribe, true, "Enable to only index subscribed accounts")

		// Profling flags
		ServerCmd.Flags().BoolVar(&profiling, flags.PprofEnable, false, "Enable the pprof HTTP server")
		ServerCmd.Flags().IntVar(&profilPort, flags.PprofPort, 8000, "pprof HTTP server listening port")
		ServerCmd.Flags().StringVar(&profilHost, flags.PprofHost, "0.0.0.0", "pprof HTTP server listening interface")

		// erc20 flags
		ServerCmd.Flags().BoolVar(&subscribeErc20token, flags.SubscribeErc20token, false, "Enable erc20 token subscription. Please specify the erc20 tokens in configs/erc20.yaml")
	} else {
		cobra.OnInitialize(initConfig)
	}
}

func initConfig() {
	viper.SetConfigType(cfgFileType)
	viper.SetConfigName(cfgFileName)
	viper.AddConfigPath(cfgFilePath)
	if err := viper.ReadInConfig(); err != nil {
		log.Crit("Can not load config file", "err", err)
	}
	loadFlagToVar()
}

func loadFlagToVar() {
	// flags for ethereum service
	ethProtocol = viper.GetString(flags.EthProtocol)
	ethHost = viper.GetString(flags.EthHost)
	ethPort = viper.GetInt(flags.EthPort)

	// flags for database
	dbDriver = viper.GetString(flags.DbDriver)
	dbHost = viper.GetString(flags.DbHost)
	dbPort = viper.GetInt(flags.DbPort)
	dbName = viper.GetString(flags.DbName)
	dbUser = viper.GetString(flags.DbUser)
	dbPassword = viper.GetString(flags.DbPassword)

	// flags for syncing
	targetBlock = viper.GetInt64(flags.SyncTargetBlock)

	//flag for pprof
	profiling = viper.GetBool(flags.PprofEnable)
	profilPort = viper.GetInt(flags.PprofPort)
	profilHost = viper.GetString(flags.PprofHost)

	// flags for enabled functions
	subscribeErc20token = viper.GetBool(flags.SubscribeErc20token)
}
