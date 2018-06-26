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

package client

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/getamis/eth-indexer/contracts"
	"github.com/getamis/eth-indexer/model"
)

//go:generate mockery -name Balancer
// Balancer is a wrapper interface to batch get balances
type Balancer interface {
	// BalanceOf returns the ERC20/Ether balances
	BalanceOf(context.Context, *big.Int, map[ethCommon.Address]map[ethCommon.Address]struct{}) (map[ethCommon.Address]map[ethCommon.Address]*big.Int, error)
}

// BalanceOf returns the erc20 balances
func (c *client) BalanceOf(ctx context.Context, blockNumber *big.Int, addrs map[ethCommon.Address]map[ethCommon.Address]struct{}) (erc20Balances map[ethCommon.Address]map[ethCommon.Address]*big.Int, err error) {
	var msgs []*ethereum.CallMsg
	mappingAddrs := make(map[ethCommon.Address][]ethCommon.Address)
	// Only handle non-ETH balances
	for erc20Addr, list := range addrs {
		if erc20Addr != model.ETHAddress {
			for addr := range list {
				// Append balance of message
				msgs = append(msgs, contracts.BalanceOfMsg(erc20Addr, addr))
				mappingAddrs[erc20Addr] = append(mappingAddrs[erc20Addr], addr)
			}
		}
	}

	// Get batch results
	outputs, err := c.BatchCallContract(ctx, msgs, blockNumber)
	if err != nil {
		return nil, err
	}

	erc20Balances = make(map[ethCommon.Address]map[ethCommon.Address]*big.Int)
	for i := 0; i < len(msgs); i++ {
		msg := msgs[i]
		contractAddr := *msg.To
		lens := len(mappingAddrs[contractAddr])
		erc20Balances[contractAddr] = make(map[ethCommon.Address]*big.Int, lens)

		// Get the length of requested address in given contract address
		for j := 0; j < lens; j++ {
			balance, err := contracts.DecodeBalanceOf(outputs[i+j])
			if err != nil {
				return nil, err
			}
			erc20Balances[contractAddr][mappingAddrs[contractAddr][j]] = balance
		}
		i = i + lens
	}

	// Handle ETH balances
	if _, ok := addrs[model.ETHAddress]; ok {
		erc20Balances[model.ETHAddress], err = c.ETHBalanceOf(ctx, blockNumber, addrs[model.ETHAddress])
		if err != nil {
			return nil, err
		}
	}
	return
}

// ethBalanceOf returns the ether balances
func (c *client) ETHBalanceOf(ctx context.Context, blockNumber *big.Int, addrs map[ethCommon.Address]struct{}) (etherBalances map[ethCommon.Address]*big.Int, err error) {
	lens := len(addrs)
	var addrList []ethCommon.Address
	for addr := range addrs {
		addrList = append(addrList, addr)
	}

	// Get ethers
	ethers, err := c.BatchBalanceAt(ctx, addrList, blockNumber)
	if err != nil {
		return nil, err
	}

	// Construct ether balances
	etherBalances = make(map[ethCommon.Address]*big.Int, lens)
	for i, e := range ethers {
		etherBalances[addrList[i]] = e
	}
	return
}
