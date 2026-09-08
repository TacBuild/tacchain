package v163

// Absolute, audited constants for the v1.6.3 rescue migration.
//
// Every address and amount here is compiled in. The handler never reads a file,
// an env var or the network at execution time - see the consensus rules in
// ai-review/163-upgrade-plan.md.

const UpgradeName = "v1.6.3"

// Denom is the staking/bank denom on TAC mainnet.
const Denom = "utac"

// ChainID gates the whole migration: on any other network the handler runs the
// module migrations only. Prevents the rescue from firing on a devnet or testnet
// that happens to reuse the upgrade name.
const ChainID = "tacchain_239-1"

// ── Task A1: old OFT escrow -> the 19 in-flight-message recipients ──────────

// EscrowAddr is the old OFT escrow contract holding the stuck refunds.
const EscrowAddr = "0x1219c409faBe2C27Bd0D1A565daeed9Bd9f271dE"

// Refund is one in-flight-message repayment, in base units (utac).
type Refund struct {
	To     string // EVM hex address
	Amount string // utac
}

// Refunds sum to RefundTotal, which equalled the escrow balance to the wei at
// height 24993121. Order is fixed and must not be sorted at runtime.
var Refunds = []Refund{
	{To: "0xb3f5a608cf1a2af390f13a385e50c2f208c96e90", Amount: "11575107000000000000000000"},
	{To: "0xfa63cc020eb9c6f5f2f3922f1e1c178679341ce3", Amount: "8195592000000000000000000"},
	{To: "0x7954bac04795c6890a2cc6b051539bd2aa225d16", Amount: "6406552196025000000000000"},
	{To: "0x150cfceb5bf4820c60b8b62135f683b05251d6f6", Amount: "3005000000000000000000000"},
	{To: "0x92b2c2f4ccd00cefa50e891ed1afe7f8590710bf", Amount: "1964911000000000000000000"},
	{To: "0xd6c0cc1b5a39967efe8672d345eb0b81f085148e", Amount: "965000000000000000000000"},
	{To: "0x79737fc740015801b735056e9535b554a99effd9", Amount: "950000000000000000000000"},
	{To: "0x296e70b628c34a5e53c760682b479a8e8c643b8a", Amount: "852400000000000000000000"},
	{To: "0xe48e1eee34fb519165a80d51265a7474d90bed31", Amount: "242345000000000000000000"},
	{To: "0xf3e1eb8a57fff797f6f6d98ab96b67c142a42d6a", Amount: "120000000000000000000000"},
	{To: "0x21929606dab7548687981ea04eaf1bf4e25cbceb", Amount: "46871000000000000000000"},
	{To: "0xb7434015ef227e6a858885214662fa12017203c0", Amount: "18613304005000000000000"},
	{To: "0x376bd7c687487ecf26e45b4d023c80948dc197a8", Amount: "10000000000000000000000"},
	{To: "0x2ea243cfa3e79f33547974d3b36ea5ab65ca68d3", Amount: "1000000000000000000000"},
	{To: "0xbb0cd18711cfee51efc196501bc848126e3da6ce", Amount: "300000000000000000000"},
	{To: "0x0c947e5e0d14b7a0ae7a3038eb4f2d9e951d0510", Amount: "200000000000000000000"},
	{To: "0x35c8b4d61aa6ae8d808952109210eb535d3147e0", Amount: "200000000000000000000"},
	{To: "0xa6b3383bde7fd8cf9f631e0a6e27c2523fe4d758", Amount: "10000000000000000000"},
	{To: "0xc7ac513d04157a1d0c8da4dbe1cd307d27f0ad01", Amount: "2000000000000000000"},
}

// RefundTotal is the sum of Refunds (34354103.50003 TAC).
const RefundTotal = "34354103500030000000000000"

// ── Task A2: fund the new OFT for ETH backing ──────────────────────────────

// The top-up was never done by hand, so this migration performs it. The large
// balance shifts seen between the two audit snapshots (new OFT +~503.4M TAC,
// Foundation multisig -~473.0M) were unrelated movements, confirmed 2026-09-08.
const (
	FoundationMultisigAddr = "0xa1257372a15e6A7F2A0bDE5d44F58233c07b9BdE"
	NewOFTAddr             = "0x3B56716bDE6Ec5E960D9b294cfc8F0527452B967"
	NewOFTFunding          = "7628902102546000000000000" // 7,628,902.102546 TAC
)

// ── Task B / C: the six compromised Ankr-operated TOE validators ───────────

// RedirectDest receives the self-unbonding stake: TOE Treasury Multisig,
// EVM infra partnerships.
const RedirectDest = "0xf610Ea937103b696F7f95cca90DE8824d978eDbF"

// TOEValidator pairs a compromised operator account with its validator. Both are
// the same 20 bytes under different bech32 prefixes; they are spelled out so the
// handler never has to re-derive one from the other.
type TOEValidator struct {
	Moniker  string
	Valoper  string // bech32 tacvaloper1...
	Operator string // bech32 tac1... (compromised)
}

// TOEValidators is the full, closed set the migration is allowed to touch.
var TOEValidators = []TOEValidator{
	{Moniker: "TOE 1", Valoper: "tacvaloper1lh6xd8x9n9jspywcpf6npke9glh56pd5qreyfd", Operator: "tac1lh6xd8x9n9jspywcpf6npke9glh56pd5a3z2y9"},
	{Moniker: "TOE 2", Valoper: "tacvaloper1a4xlewuye9uvjyp4s4yklkvg3pkrua02ms047v", Operator: "tac1a4xlewuye9uvjyp4s4yklkvg3pkrua02xz5mny"},
	{Moniker: "TOE 3", Valoper: "tacvaloper12f47q9tdgrmc7adulqgap0uhxukflhhg2pxxnw", Operator: "tac12f47q9tdgrmc7adulqgap0uhxukflhhghnag7x"},
	{Moniker: "TOE 4", Valoper: "tacvaloper10q6xlum6mz6ndn54280wt5phqkxcsqcqnmmu98", Operator: "tac10q6xlum6mz6ndn54280wt5phqkxcsqcqwfqjg0"},
	{Moniker: "TOE 5", Valoper: "tacvaloper1pdu86gjvnnr2786xtkw2eggxkmrsur0z0fpztm", Operator: "tac1pdu86gjvnnr2786xtkw2eggxkmrsur0zjm6vxn"},
	{Moniker: "TOE 6", Valoper: "tacvaloper13th093g5nmlche3ljpfar87z88ah3u0mvnka8f", Operator: "tac13th093g5nmlche3ljpfar87z88ah3u0m3pdn2p"},
}

// SelfStakeExpected is what the six accounts held in unbonding at height
// 24993121 (19979613.401007160544395447 TAC). Recorded for the summary event ONLY - it is never
// compared with an abort, because anyone can move the actual figure.
const SelfStakeExpected = "19979613401007161764310628"
