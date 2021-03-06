package vm

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/wanchain/go-wanchain/common"
	"github.com/wanchain/go-wanchain/crypto"
	"github.com/wanchain/go-wanchain/log"
	"math/big"
	"math/rand"
)

// OTABalance2ContractAddr convert ota balance to ota storage address
//
// 1 wancoin --> (bigint)1000000000000000000 --> "0x0000000000000000000001000000000000000000"
//
func OTABalance2ContractAddr(balance *big.Int) common.Address {
	if balance == nil {
		return common.Address{}
	}

	return common.HexToAddress(balance.String())
	//	return common.BigToAddress(balance)
}

// GetAXFromWanAddr retrieve ota AX from ota WanAddr
func GetAXFromWanAddr(otaWanAddr []byte) ([]byte, error) {
	if len(otaWanAddr) != common.WAddressLength {
		return nil, errors.New("invalid input param!")
	}

	return otaWanAddr[1 : 1+common.HashLength], nil
}

// GetOtaBalanceFromAX retrieve ota balance from ota AX
func GetOtaBalanceFromAX(statedb StateDB, otaAX []byte) (*big.Int, error) {
	if statedb == nil || len(otaAX) != common.HashLength {
		return nil, errors.New("invalid input param!")
	}

	balance := statedb.GetStateByteArray(otaBalanceStorageAddr, common.BytesToHash(otaAX))
	if len(balance) == 0 {
		return common.Big0, nil
	}

	return new(big.Int).SetBytes(balance), nil
}

// SetOtaBalanceToAX set ota balance as 'balance'. Overwrite if ota balance exit already.
func SetOtaBalanceToAX(statedb StateDB, otaAX []byte, balance *big.Int) error {
	if statedb == nil || len(otaAX) != common.HashLength || balance == nil {
		return errors.New("invalid input param!")
	}

	statedb.SetStateByteArray(otaBalanceStorageAddr, common.BytesToHash(otaAX), balance.Bytes())
	return nil
}

// GetOtaBalanceFromWanAddr retrieve ota balance from ota WanAddr
func GetOtaBalanceFromWanAddr(statedb StateDB, otaWanAddr []byte) (*big.Int, error) {
	if statedb == nil || len(otaWanAddr) != common.WAddressLength {
		return nil, errors.New("invalid input param!")
	}

	otaAX, _ := GetAXFromWanAddr(otaWanAddr)
	balance := statedb.GetStateByteArray(otaBalanceStorageAddr, common.BytesToHash(otaAX))
	if len(balance) == 0 {
		return common.Big0, nil
	}

	return new(big.Int).SetBytes(balance), nil
}

// ChechOTAExit checks the OTA exit or not
func CheckOTAExit(statedb StateDB, otaAX []byte) (exit bool, balance *big.Int, err error) {
	if statedb == nil || len(otaAX) < common.HashLength {
		return false, nil, errors.New("invalid input param!")
	}

	otaAddrKey := common.BytesToHash(otaAX)
	balance, err = GetOtaBalanceFromAX(statedb, otaAddrKey[:])
	if err != nil {
		return false, nil, err
	} else if balance.Cmp(common.Big0) == 0 {
		return false, nil, nil
	}

	mptAddr := OTABalance2ContractAddr(balance)

	otaValue := statedb.GetStateByteArray(mptAddr, otaAddrKey)
	if otaValue != nil && len(otaValue) != 0 {
		return true, balance, nil
	}

	return false, nil, nil
}

// BatCheckOTAExit batch check the OTAs exit or not.
//
// return true means all OTAs exit and their have same balance
//
func BatCheckOTAExit(statedb StateDB, otaAXs [][]byte) (exit bool, balance *big.Int, unexitOta []byte, err error) {
	if statedb == nil || len(otaAXs) == 0 {
		return false, nil, nil, errors.New("invalid input param!")
	}

	for _, otaAX := range otaAXs {
		if len(otaAX) < common.HashLength {
			return false, nil, otaAX, errors.New("invalid input ota AX!")
		}

		balanceTmp, err := GetOtaBalanceFromAX(statedb, otaAX[:common.HashLength])
		if err != nil {
			return false, nil, otaAX, err
		} else if balanceTmp.Cmp(common.Big0) == 0 {
			return false, nil, otaAX, errors.New("ota balance is 0! ota:" + common.ToHex(otaAX))
		} else if balance == nil {
			balance = balanceTmp
			continue
		} else if balance.Cmp(balanceTmp) != 0 {
			return false, nil, otaAX, errors.New("otas have different balances! ota:" + common.ToHex(otaAX))
		}
	}

	mptAddr := OTABalance2ContractAddr(balance)
	for _, otaAX := range otaAXs {
		otaAddrKey := common.BytesToHash(otaAX)
		otaValue := statedb.GetStateByteArray(mptAddr, otaAddrKey)
		if len(otaValue) == 0 {
			return false, nil, otaAX, errors.New("ota doesn't exit:" + common.ToHex(otaAX))
		}
	}

	return true, balance, nil, nil
}

// setOTA storage ota info, include balance and WanAddr. Overwrite if ota exit already.
func setOTA(statedb StateDB, balance *big.Int, otaWanAddr []byte) error {
	if statedb == nil || balance == nil || len(otaWanAddr) != common.WAddressLength {
		return errors.New("invalid input param!")
	}

	otaAX, _ := GetAXFromWanAddr(otaWanAddr)
	//balanceOld, err := GetOtaBalanceFromAX(statedb, otaAX)
	//if err != nil {
	//	return err
	//}
	//
	//if balanceOld != nil && balanceOld.Cmp(common.Big0) != 0 {
	//	return errors.New("ota balance is not 0! old balance:" + balanceOld.String())
	//}

	mptAddr := OTABalance2ContractAddr(balance)
	statedb.SetStateByteArray(mptAddr, common.BytesToHash(otaAX), otaWanAddr)
	return SetOtaBalanceToAX(statedb, otaAX, balance)
}

// AddOTAIfNotExit storage ota info if doesn't exit already.
func AddOTAIfNotExit(statedb StateDB, balance *big.Int, otaWanAddr []byte) (bool, error) {
	if statedb == nil || balance == nil || len(otaWanAddr) != common.WAddressLength {
		return false, errors.New("invalid input param!")
	}

	otaAX, _ := GetAXFromWanAddr(otaWanAddr)
	otaAddrKey := common.BytesToHash(otaAX)
	exit, _, err := CheckOTAExit(statedb, otaAddrKey[:])
	if err != nil {
		return false, err
	}

	if exit {
		return false, errors.New("ota exit already!")
	}

	err = setOTA(statedb, balance, otaWanAddr)
	if err != nil {
		return false, err
	}

	return true, nil
}

// GetOTAInfoFromAX retrieve ota info, include balance and WanAddr
func GetOTAInfoFromAX(statedb StateDB, otaAX []byte) (otaWanAddr []byte, balance *big.Int, err error) {
	if statedb == nil || len(otaAX) < common.HashLength {
		return nil, nil, errors.New("invalid input param!")
	}

	otaAddrKey := common.BytesToHash(otaAX)
	balance, err = GetOtaBalanceFromAX(statedb, otaAddrKey[:])
	if err != nil {
		return nil, nil, err
	}

	if balance == nil || balance.Cmp(common.Big0) == 0 {
		return nil, nil, errors.New("ota balance is 0!")
	}

	mptAddr := OTABalance2ContractAddr(balance)

	otaValue := statedb.GetStateByteArray(mptAddr, otaAddrKey)
	if otaValue != nil && len(otaValue) != 0 {
		return otaValue, balance, nil
	}

	return nil, balance, nil
}

// OTAInSet checks ota in set or not
func OTAInSet(otaSet [][]byte, ota []byte) bool {
	for _, exit := range otaSet {
		if bytes.Equal(exit, ota) {
			return true
		}
	}

	return false
}

// GetOTASet retrieve the setNum of same balance OTA address of the input OTA setting by otaAX.
// As far as possible return the OTA set does not contain duplicate items and input otaAX self.
func GetOTASet(statedb StateDB, otaAX []byte, setNum int) (otaWanAddrs [][]byte, balance *big.Int, err error) {
	if statedb == nil || len(otaAX) != common.HashLength {
		return nil, nil, errors.New("invalid input param!")
	}

	balance, err = GetOtaBalanceFromAX(statedb, otaAX)
	if err != nil {
		return nil, nil, err
	} else if balance == nil || balance.Cmp(common.Big0) == 0 {
		return nil, nil, errors.New("cant find ota address balance!")
	}

	mptAddr := OTABalance2ContractAddr(balance)
	log.Debug("GetOTASet", "mptAddr", common.ToHex(mptAddr[:]))

	otaWanAddrs = make([][]byte, 0)
	rnd := rand.Intn(100) + 1

	i := 0
	loopTimes := 0
	passNum := 0
	mptEleCount := 0
	randomSel := func(value []byte) bool {
		loopTimes++
		if loopTimes%rnd == 0 {
			otaWanAddrs = append(otaWanAddrs, value)
			i++
			rnd = rand.Intn(100) + 1

			// not enough, return true to continue
			return i < setNum
		} else {
			// return true to continue
			return true
		}
	}

	isSelf := func(value []byte) bool {
		findAX, _ := GetAXFromWanAddr(value)
		return bytes.Equal(findAX, otaAX)
	}

	for {
		statedb.ForEachStorageByteArray(mptAddr, func(key common.Hash, value []byte) bool {
			if len(value) != common.WAddressLength {
				log.Error("invalid OTA address!", "balance", balance, "value", value)
				err = errors.New(fmt.Sprint("invalid OTA address! balance:", balance, ", ota:", value))
				return false
			}

			if passNum == 0 {
				// first pass, record total count
				mptEleCount++

				// find self, continue
				if isSelf(value) {
					return true
				}

				// random select
				return randomSel(value)

			} else if passNum == 1 {
				// second pass
				if setNum >= mptEleCount {
					if !OTAInSet(otaWanAddrs, value) {
						// mpt ele less than set num, reap the one don't exit in set
						otaWanAddrs = append(otaWanAddrs, value)
						i++

						// not enough, return true to continue
						return i < setNum
					} else {
						// return true to continue
						return true
					}
				} else if !OTAInSet(otaWanAddrs, value) {
					// find self, continue
					if isSelf(value) {
						return true
					}

					// random select
					return randomSel(value)
				} else {
					return true
				}

			} else {
				// third or more pass
				if setNum >= mptEleCount {
					// random select
					return randomSel(value)
				} else if !OTAInSet(otaWanAddrs, value) {
					// find self, continue
					if isSelf(value) {
						return true
					}

					// random select
					return randomSel(value)
				} else {
					return true
				}
			}
		})

		if err != nil {
			return nil, nil, err
		}

		if mptEleCount == 0 {
			return nil, balance, errors.New("no ota address exit! balance:" + balance.String())
		}

		if i >= setNum {
			return otaWanAddrs, balance, nil
		}

		passNum++
	}
}

// CheckOTAImageExit checks ota image key exit already or not
func CheckOTAImageExit(statedb StateDB, otaImage []byte) (bool, []byte, error) {
	if statedb == nil || len(otaImage) == 0 {
		return false, nil, errors.New("invalid input param!")
	}

	otaImageKey := crypto.Keccak256Hash(otaImage)
	otaImageValue := statedb.GetStateByteArray(otaImageStorageAddr, otaImageKey)
	if otaImageValue != nil && len(otaImageValue) != 0 {
		return true, otaImageValue, nil
	}

	return false, nil, nil
}

// AddOTAImage storage ota image key. Overwrite if exit already.
func AddOTAImage(statedb StateDB, otaImage []byte, value []byte) error {
	if statedb == nil || len(otaImage) == 0 || len(value) == 0 {
		return errors.New("invalid input param!")
	}

	otaImageKey := crypto.Keccak256Hash(otaImage)
	statedb.SetStateByteArray(otaImageStorageAddr, otaImageKey, value)
	return nil
}
