package hd

import (
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
)

// NewExtendedKey
func NewExtendedKey() (*hdkeychain.ExtendedKey, error) {
	return hdkeychain.NewMaster([]byte{}, &chaincfg.MainNetParams)
}
