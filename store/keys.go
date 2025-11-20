package store

import (
	"fmt"
)

// -----------------------------------------------------------------------------
// Top-level domain prefixes
// -----------------------------------------------------------------------------

const (
	// DDEX primary objects
	PrefixERN  = "ern/"
	PrefixPIE  = "pie/"
	PrefixMEAD = "mead/"

	// CID objects
	PrefixCID = "cid/"

	// Accounts and balances
	PrefixAccount = "account/"

	// Grants
	PrefixGrant = "grant/"

	// Secondary indexes
	PrefixIdxThread    = "index/thread/"
	PrefixIdxMessageID = "index/message-id/"
	PrefixIdxSender    = "index/sender/"
	PrefixIdxBlock     = "index/block/"
	PrefixIdxGrant     = "index/grant/grantee/"
	PrefixIdxCIDToERN  = "index/cid/ern/"
	PrefixIdxERNToCID  = "index/ern/cid/"
)

// -----------------------------------------------------------------------------
// Helpers to convert string → []byte without repeated boilerplate
// -----------------------------------------------------------------------------

func b(s string) []byte { return []byte(s) }

// -----------------------------------------------------------------------------
// DDEX Primary Keys (ERN / PIE / MEAD)
// -----------------------------------------------------------------------------

// ERN/{ern_address}/msg/{message_id}
func KeyERNMessage(ernAddress, messageID string) []byte {
	return b(fmt.Sprintf("%s%s/msg/%s", PrefixERN, ernAddress, messageID))
}

// PIE/{pie_address}/msg/{message_id}
func KeyPIEMessage(pieAddress, messageID string) []byte {
	return b(fmt.Sprintf("%s%s/msg/%s", PrefixPIE, pieAddress, messageID))
}

// MEAD/{mead_address}/msg/{message_id}
func KeyMEADMessage(meadAddress, messageID string) []byte {
	return b(fmt.Sprintf("%s%s/msg/%s", PrefixMEAD, meadAddress, messageID))
}

// -----------------------------------------------------------------------------
// Thread Index: index/thread/{thread_id}/{message_id}
// -----------------------------------------------------------------------------

func KeyIndexThread(threadID, messageID string) []byte {
	return b(fmt.Sprintf("%s%s/%s", PrefixIdxThread, threadID, messageID))
}

// -----------------------------------------------------------------------------
// MessageID Index: index/message-id/{message_id}
// -----------------------------------------------------------------------------

func KeyIndexMessageID(messageID string) []byte {
	return b(fmt.Sprintf("%s%s", PrefixIdxMessageID, messageID))
}

// -----------------------------------------------------------------------------
// Sender Index: index/sender/{sender}/{object_address}/{message_id}
// -----------------------------------------------------------------------------

func KeyIndexSender(sender, objectAddress, messageID string) []byte {
	return b(fmt.Sprintf("%s%s/%s/%s",
		PrefixIdxSender, sender, objectAddress, messageID))
}

// -----------------------------------------------------------------------------
// Block Index: index/block/{block_height}/{object_address}/{message_id}
// block_height must already be zero-padded if ordering matters
// -----------------------------------------------------------------------------

func KeyIndexBlock(heightStr, objectAddress, messageID string) []byte {
	return b(fmt.Sprintf("%s%s/%s/%s",
		PrefixIdxBlock, heightStr, objectAddress, messageID))
}

// -----------------------------------------------------------------------------
// Accounts: account/{address}/data | nonce | balance
// -----------------------------------------------------------------------------

func KeyAccountData(address string) []byte {
	return b(fmt.Sprintf("%s%s/data", PrefixAccount, address))
}

func KeyAccountNonce(address string) []byte {
	return b(fmt.Sprintf("%s%s/nonce", PrefixAccount, address))
}

func KeyAccountBalance(address string) []byte {
	return b(fmt.Sprintf("%s%s/balance", PrefixAccount, address))
}

// -----------------------------------------------------------------------------
// Grants
// grant/{owner}/{grantee}
// -----------------------------------------------------------------------------

func KeyGrant(owner, grantee string) []byte {
	return b(fmt.Sprintf("%s%s/%s", PrefixGrant, owner, grantee))
}

// index/grant/grantee/{grantee}/{owner}
func KeyIndexGrantByGrantee(grantee, owner string) []byte {
	return b(fmt.Sprintf("%s%s/%s", PrefixIdxGrant, grantee, owner))
}

// -----------------------------------------------------------------------------
// CID Primary Storage: cid/{cid}/data
// -----------------------------------------------------------------------------

func KeyCIDData(cid string) []byte {
	return b(fmt.Sprintf("%s%s/data", PrefixCID, cid))
}

// -----------------------------------------------------------------------------
// CID ↔ ERN Cross Indexes
// -----------------------------------------------------------------------------

// index/cid/ern/{cid}/{ern_address}
// → primary ERN key
func KeyIndexCIDToERN(cid, ernAddress string) []byte {
	return b(fmt.Sprintf("%s%s/%s", PrefixIdxCIDToERN, cid, ernAddress))
}
