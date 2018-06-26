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

package subscription

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/getamis/eth-indexer/model"
	"github.com/jinzhu/gorm"
)

//go:generate mockery -name Store
type Store interface {
	Insert(data *model.Subscription) error
	UpdateBlockNumber(data *model.Subscription) error
	Find(blockNumber int64) (result []*model.Subscription, err error)
	FindByAddresses(addrs [][]byte) (result []*model.Subscription, err error)

	// Total balance
	InsertTotalBalance(data *model.TotalBalance) error
	FindTotalBalance(blockNumber int64, token common.Address, group int64) (result *model.TotalBalance, err error)

	Reset(from, to int64) error
}

type store struct {
	db *gorm.DB
}

func NewWithDB(db *gorm.DB) Store {
	return &store{
		db: db,
	}
}

func (t *store) Insert(data *model.Subscription) error {
	return t.db.Create(data).Error
}

func (t *store) UpdateBlockNumber(data *model.Subscription) error {
	return t.db.Model(data).Update(data).Error
}

func (t *store) Find(blockNumber int64) (result []*model.Subscription, err error) {
	err = t.db.Where("block_number = ?", blockNumber).Find(&result).Error
	return
}

func (t *store) FindByAddresses(addrs [][]byte) (result []*model.Subscription, err error) {
	if len(addrs) == 0 {
		return []*model.Subscription{}, nil
	}

	var tmp []*model.Subscription
	db := t.db
	for _, addr := range addrs {
		db = db.Or("address = ?", addr)
	}
	err = db.Find(&tmp).Error

	// Exclude block number is 0 (it means the subscription is not enabled)
	if err == nil {
		for i, r := range tmp {
			if r.BlockNumber != 0 {
				result = append(result, tmp[i])
			}
		}
	}
	return
}

func (t *store) InsertTotalBalance(data *model.TotalBalance) error {
	return t.db.Create(data).Error
}

func (t *store) FindTotalBalance(blockNumber int64, token common.Address, group int64) (result *model.TotalBalance, err error) {
	result = &model.TotalBalance{}
	err = t.db.Where("block_number <= ? AND token = ? AND `group` = ?", blockNumber, token.Bytes(), group).Order("block_number DESC").Limit(1).Find(&result).Error
	// if not found error, hide error and return total balance = 0
	if err == gorm.ErrRecordNotFound {
		err = nil
		result = &model.TotalBalance{
			BlockNumber: blockNumber,
			Token:       token.Bytes(),
			Group:       group,
			Balance:     "0",
		}
	}
	return
}

func (t *store) Reset(from, to int64) error {
	// Set the block number of subscription to 0
	results := []*model.Subscription{}
	err := t.db.Where("block_number >= ? AND block_number <= ?", from, to).Find(&results).Error
	if err != nil {
		return err
	}

	for _, r := range results {
		r.BlockNumber = 0
		err = t.UpdateBlockNumber(r)
		if err != nil {
			return err
		}
	}

	// Delete total balances
	return t.db.Delete(model.TotalBalance{}, "block_number >= ? AND block_number <= ?", from, to).Error
}
