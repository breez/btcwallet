package wallet

import (
	"encoding/hex"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/walletdb"
	"github.com/btcsuite/btcwallet/wtxmgr"
)

func (w *Wallet) GetUtxosFromHeight(net *chaincfg.Params, start int32, address string) ([]Utxo, error) {
	var txos []Utxo
	outPoints := make(map[string]struct{})
	spentOutPoints := make(map[string]struct{})
	err := walletdb.View(w.db, func(dbtx walletdb.ReadTx) error {
		txmgrNs := dbtx.ReadBucket(wtxmgrNamespaceKey)
		rangeFn := func(details []wtxmgr.TxDetails) (bool, error) {
			// TODO: probably should make RangeTransactions not reuse the
			// details backing array memory.
			dets := make([]wtxmgr.TxDetails, len(details))
			copy(dets, details)
			details = dets

			for _, d := range details {
				if d.Block.Height != -1 {
					for i, txout := range d.MsgTx.TxOut {
						_, addrs, _, err := txscript.ExtractPkScriptAddrs(txout.PkScript, net)
						if err == nil {
							if len(addrs) == 0 {
								if len(txout.PkScript) == 0 {
									log.Errorf("found script with zero addresses and zero length")
									continue
								}
								log.Warnf("found script with zero addresses %v", hex.EncodeToString(txout.PkScript))
								dis, err := txscript.DisasmString(txout.PkScript)
								if err != nil {
									log.Errorf("unable to parse script")
								} else {
									log.Infof("parsed script: %v", dis)
								}

								continue
							}
							if addrs[0].String() == address {
								h := d.MsgTx.TxHash()
								op := wire.NewOutPoint(&h, uint32(i))
								txos = append(txos, Utxo{
									Value:       btcutil.Amount(txout.Value),
									BlockHeight: d.Block.Height,
									OutPoint:    *op,
								})
								outPoints[op.String()] = struct{}{}
								//return true, nil
							}
						}
					}
					for _, txin := range d.MsgTx.TxIn {
						if _, ok := outPoints[txin.PreviousOutPoint.String()]; ok {
							spentOutPoints[txin.PreviousOutPoint.String()] = struct{}{}
						}
					}
				}
			}
			return false, nil
		}

		return w.TxStore.RangeTransactions(txmgrNs, start, int32(^uint32(0)>>1), rangeFn)
	})
	if err != nil {
		return nil, err
	}
	var utxos []Utxo
	for _, txo := range txos {
		if _, ok := spentOutPoints[txo.OutPoint.String()]; !ok {
			utxos = append(utxos, txo)
		}
	}
	return utxos, nil
}
