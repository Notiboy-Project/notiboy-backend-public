package xrpl

type TxnReferenceRequestParams struct {
	Transaction string `json:"transaction"`
	Binary      bool   `json:"binary"`
}
type TxnReferenceRequest struct {
	Method string                      `json:"method"`
	Params []TxnReferenceRequestParams `json:"params"`
}

type TxnReferenceResponse struct {
	Result struct {
		Account            string `json:"Account"`
		Amount             string `json:"Amount"`
		Destination        string `json:"Destination"`
		Fee                string `json:"Fee"`
		LastLedgerSequence int    `json:"LastLedgerSequence"`
		Memos              []struct {
			Memo struct {
				MemoData   string `json:"MemoData"`
				MemoFormat string `json:"MemoFormat"`
				MemoType   string `json:"MemoType"`
			} `json:"Memo"`
		} `json:"Memos"`
		Sequence        int    `json:"Sequence"`
		SigningPubKey   string `json:"SigningPubKey"`
		TransactionType string `json:"TransactionType"`
		TxnSignature    string `json:"TxnSignature"`
		Date            int    `json:"date"`
		Hash            string `json:"hash"`
		LedgerIndex     int    `json:"ledger_index"`
		Meta            struct {
			AffectedNodes []struct {
				ModifiedNode struct {
					FinalFields struct {
						Account        string `json:"Account"`
						Balance        string `json:"Balance"`
						Domain         string `json:"Domain"`
						EmailHash      string `json:"EmailHash"`
						Flags          int    `json:"Flags"`
						OwnerCount     int    `json:"OwnerCount"`
						Sequence       int    `json:"Sequence"`
						BurnedNFTokens int    `json:"BurnedNFTokens,omitempty"`
						MintedNFTokens int    `json:"MintedNFTokens,omitempty"`
					} `json:"FinalFields"`
					LedgerEntryType string `json:"LedgerEntryType"`
					LedgerIndex     string `json:"LedgerIndex"`
					PreviousFields  struct {
						Balance  string `json:"Balance"`
						Sequence int    `json:"Sequence,omitempty"`
					} `json:"PreviousFields"`
					PreviousTxnID     string `json:"PreviousTxnID"`
					PreviousTxnLgrSeq int    `json:"PreviousTxnLgrSeq"`
				} `json:"ModifiedNode"`
			} `json:"AffectedNodes"`
			TransactionIndex  int    `json:"TransactionIndex"`
			TransactionResult string `json:"TransactionResult"`
			DeliveredAmount   string `json:"delivered_amount"`
		} `json:"meta"`
		Status    string `json:"status"`
		Validated bool   `json:"validated"`
	} `json:"result"`
	Warnings []struct {
		Id      int    `json:"id"`
		Message string `json:"message"`
	} `json:"warnings"`
}

type PricingAPIResponse struct {
	Ripple struct {
		Usd float64 `json:"usd"`
	} `json:"ripple"`
}
