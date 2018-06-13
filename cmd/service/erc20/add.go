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

package erc20

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/getamis/eth-indexer/service/pb"
	"github.com/getamis/sirius/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	cfgFileName string = "erc20"
	cfgFileType string = "yaml"
	cfgFilePath string = "./configs"
)

var (
	host string
	port int
	list map[string]interface{}
)

var AddCmd = &cobra.Command{
	Use:   "add",
	Short: "erc20 sends add erc20 token request to rpc",
	Long:  `erc20 sends add erc20 token request to rpc`,
	RunE: func(cmd *cobra.Command, args []string) error {
		conn, err := grpc.Dial(fmt.Sprintf("%s:%d", host, port), grpc.WithInsecure())
		if err != nil {
			log.Error("Failed to connect to gRPC", "err", err)
			return err
		}
		client := pb.NewERC20ServiceClient(conn)
		ctx := context.Background()

		for _, v := range list {
			data, _ := json.Marshal(v)
			result := make(map[string]string)
			err := json.Unmarshal(data, &result)

			address := result["address"]
			block, _ := strconv.ParseInt(result["block"], 10, 64)

			res, err := client.AddERC20(ctx, &pb.AddERC20Request{
				Address:     address,
				BlockNumber: int64(block),
			})
			if err != nil {
				return err
			}

			fmt.Printf("ERC20 contract is added, address = %v, block number = %v, name = %v, decimals = %v, total supply = %v", res.Address, res.BlockNumber, res.Name, res.Decimals, res.TotalSupply)

		}

		return nil
	},
}

func initConfig() {
	viper.SetConfigType(cfgFileType)
	viper.SetConfigName(cfgFileName)
	viper.AddConfigPath(cfgFilePath)
	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Can't read config:", err)
		os.Exit(1)
	}
	loadFlagToVar()
}

func loadFlagToVar() {
	list = viper.GetStringMap(cfgFileName)
}
