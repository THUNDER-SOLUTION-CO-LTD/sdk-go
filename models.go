package thunder

import (
	"encoding/base64"
	"encoding/json"
)

// AccountType identifies the legal owner of a receiving account.
type AccountType string

const (
	AccountTypeNatural  AccountType = "NATURAL"
	AccountTypeJuristic AccountType = "JURISTIC"
)

// QRType selects the QR format accepted by the API.
type QRType string

const (
	QRTypePromptPay   QRType = "PROMPTPAY"
	QRTypeBillPayment QRType = "BILL_PAYMENT"
	QRTypeKShop       QRType = "KSHOP"
	QRTypeMaeManee    QRType = "MAE_MANEE"
	QRTypeTungngern   QRType = "TUNGNGERN"
)

// Info contains typed wire fields.
type Info struct {
	Meta        ResponseMeta    `json:"-"`
	Application InfoApplication `json:"application"`
	Branch      InfoBranch      `json:"branch"`
	Account     InfoAccount     `json:"account"`
	Product     InfoProduct     `json:"product"`
}

// MarshalJSON preserves absent fields on Info.
func (v Info) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// InfoApplication contains typed wire fields.
type InfoApplication struct {
	Meta      ResponseMeta     `json:"-"`
	Name      string           `json:"name"`
	AutoRenew AutoRenew        `json:"autoRenew"`
	Quota     ApplicationQuota `json:"quota"`
}

// MarshalJSON preserves absent fields on InfoApplication.
func (v InfoApplication) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// AutoRenew contains typed wire fields.
type AutoRenew struct {
	Meta      ResponseMeta `json:"-"`
	Expired   bool         `json:"expired"`
	Quota     bool         `json:"quota"`
	CreatedAt string       `json:"createdAt"`
	ExpiresAt string       `json:"expiresAt"`
}

// MarshalJSON preserves absent fields on AutoRenew.
func (v AutoRenew) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// ApplicationQuota contains typed wire fields.
type ApplicationQuota struct {
	Meta      ResponseMeta `json:"-"`
	Used      int          `json:"used"`
	Max       *int         `json:"max"`
	Remaining *int         `json:"remaining"`
	TotalUsed int          `json:"totalUsed"`
}

// MarshalJSON preserves absent fields on ApplicationQuota.
func (v ApplicationQuota) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// InfoBranch contains typed wire fields.
type InfoBranch struct {
	Meta     ResponseMeta `json:"-"`
	Name     string       `json:"name"`
	IsActive bool         `json:"isActive"`
	Quota    BranchQuota  `json:"quota"`
}

// MarshalJSON preserves absent fields on InfoBranch.
func (v InfoBranch) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BranchQuota contains typed wire fields.
type BranchQuota struct {
	Meta      ResponseMeta `json:"-"`
	Used      int          `json:"used"`
	TotalUsed int          `json:"totalUsed"`
}

// MarshalJSON preserves absent fields on BranchQuota.
func (v BranchQuota) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// InfoAccount contains typed wire fields.
type InfoAccount struct {
	Meta   ResponseMeta `json:"-"`
	Email  string       `json:"email"`
	Credit float64      `json:"credit"`
}

// MarshalJSON preserves absent fields on InfoAccount.
func (v InfoAccount) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// InfoProduct contains typed wire fields.
type InfoProduct struct {
	Meta ResponseMeta `json:"-"`
	Name string       `json:"name"`
}

// MarshalJSON preserves absent fields on InfoProduct.
func (v InfoProduct) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BankSlipResult contains typed wire fields.
type BankSlipResult struct {
	Meta            ResponseMeta           `json:"-"`
	Remark          Field[string]          `json:"remark"`
	IsDuplicate     Field[bool]            `json:"isDuplicate"`
	MatchedAccount  Field[*MatchedAccount] `json:"matchedAccount"`
	AmountInOrder   Field[float64]         `json:"amountInOrder"`
	AmountInSlip    float64                `json:"amountInSlip"`
	IsAmountMatched Field[bool]            `json:"isAmountMatched"`
	RawSlip         BankSlip               `json:"rawSlip"`
}

// MarshalJSON preserves absent fields on BankSlipResult.
func (v BankSlipResult) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// TrueWalletResult contains typed wire fields.
type TrueWalletResult struct {
	Meta            ResponseMeta           `json:"-"`
	Remark          Field[string]          `json:"remark"`
	IsDuplicate     Field[bool]            `json:"isDuplicate"`
	MatchedAccount  Field[*MatchedAccount] `json:"matchedAccount"`
	AmountInOrder   Field[float64]         `json:"amountInOrder"`
	AmountInSlip    float64                `json:"amountInSlip"`
	IsAmountMatched Field[bool]            `json:"isAmountMatched"`
	RawSlip         TrueWalletSlip         `json:"rawSlip"`
}

// MarshalJSON preserves absent fields on TrueWalletResult.
func (v TrueWalletResult) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BankSlip contains typed wire fields.
type BankSlip struct {
	Meta        ResponseMeta   `json:"-"`
	Payload     string         `json:"payload"`
	TransRef    string         `json:"transRef"`
	Date        string         `json:"date"`
	CountryCode string         `json:"countryCode"`
	Amount      SlipAmount     `json:"amount"`
	Fee         Field[float64] `json:"fee"`
	Ref1        Field[string]  `json:"ref1"`
	Ref2        Field[string]  `json:"ref2"`
	Ref3        Field[string]  `json:"ref3"`
	Sender      SlipParty      `json:"sender"`
	Receiver    SlipReceiver   `json:"receiver"`
}

// MarshalJSON preserves absent fields on BankSlip.
func (v BankSlip) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipAmount contains typed wire fields.
type SlipAmount struct {
	Meta   ResponseMeta `json:"-"`
	Amount float64      `json:"amount"`
	Local  LocalAmount  `json:"local"`
}

// MarshalJSON preserves absent fields on SlipAmount.
func (v SlipAmount) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// LocalAmount contains typed wire fields.
type LocalAmount struct {
	Meta     ResponseMeta   `json:"-"`
	Amount   Field[float64] `json:"amount"`
	Currency Field[string]  `json:"currency"`
}

// MarshalJSON preserves absent fields on LocalAmount.
func (v LocalAmount) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipReceiver contains typed wire fields.
type SlipReceiver struct {
	Meta       ResponseMeta            `json:"-"`
	Bank       Field[SlipReceiverBank] `json:"bank"`
	Account    SlipReceiverAccount     `json:"account"`
	MerchantID Field[*string]          `json:"merchantId"`
}

// MarshalJSON preserves absent fields on SlipReceiver.
func (v SlipReceiver) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipReceiverBank contains typed wire fields.
type SlipReceiverBank struct {
	Meta  ResponseMeta  `json:"-"`
	ID    Field[string] `json:"id"`
	Name  Field[string] `json:"name"`
	Short Field[string] `json:"short"`
}

// MarshalJSON preserves absent fields on SlipReceiverBank.
func (v SlipReceiverBank) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipReceiverAccount contains typed wire fields.
type SlipReceiverAccount struct {
	Meta  ResponseMeta            `json:"-"`
	Name  SlipReceiverAccountName `json:"name"`
	Bank  Field[AccountNumber]    `json:"bank"`
	Proxy Field[AccountNumber]    `json:"proxy"`
}

// MarshalJSON preserves absent fields on SlipReceiverAccount.
func (v SlipReceiverAccount) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipReceiverAccountName contains typed wire fields.
type SlipReceiverAccountName struct {
	Meta ResponseMeta  `json:"-"`
	Th   Field[string] `json:"th"`
	En   Field[string] `json:"en"`
}

// MarshalJSON preserves absent fields on SlipReceiverAccountName.
func (v SlipReceiverAccountName) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipParty contains typed wire fields.
type SlipParty struct {
	Meta    ResponseMeta    `json:"-"`
	Bank    Field[SlipBank] `json:"bank"`
	Account SlipAccount     `json:"account"`
}

// MarshalJSON preserves absent fields on SlipParty.
func (v SlipParty) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipBank contains typed wire fields.
type SlipBank struct {
	Meta  ResponseMeta  `json:"-"`
	ID    Field[string] `json:"id"`
	Name  Field[string] `json:"name"`
	Short Field[string] `json:"short"`
}

// MarshalJSON preserves absent fields on SlipBank.
func (v SlipBank) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// SlipAccount contains typed wire fields.
type SlipAccount struct {
	Meta  ResponseMeta         `json:"-"`
	Name  AccountName          `json:"name"`
	Bank  Field[AccountNumber] `json:"bank"`
	Proxy Field[AccountNumber] `json:"proxy"`
}

// MarshalJSON preserves absent fields on SlipAccount.
func (v SlipAccount) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// AccountName contains typed wire fields.
type AccountName struct {
	Meta ResponseMeta  `json:"-"`
	Th   Field[string] `json:"th"`
	En   Field[string] `json:"en"`
}

// MarshalJSON preserves absent fields on AccountName.
func (v AccountName) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// AccountNumber contains typed wire fields.
type AccountNumber struct {
	Meta    ResponseMeta `json:"-"`
	Type    string       `json:"type"`
	Account string       `json:"account"`
}

// MarshalJSON preserves absent fields on AccountNumber.
func (v AccountNumber) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// MatchedAccount contains typed wire fields.
type MatchedAccount struct {
	Meta       ResponseMeta `json:"-"`
	Bank       MatchedBank  `json:"bank"`
	NameTh     string       `json:"nameTh"`
	NameEn     string       `json:"nameEn"`
	Type       string       `json:"type"`
	BankNumber string       `json:"bankNumber"`
}

// MarshalJSON preserves absent fields on MatchedAccount.
func (v MatchedAccount) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// MatchedBank contains typed wire fields.
type MatchedBank struct {
	Meta      ResponseMeta  `json:"-"`
	NameTh    Field[string] `json:"nameTh"`
	NameEn    Field[string] `json:"nameEn"`
	Code      Field[string] `json:"code"`
	ShortCode Field[string] `json:"shortCode"`
}

// MarshalJSON preserves absent fields on MatchedBank.
func (v MatchedBank) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// TrueWalletSlip contains typed wire fields.
type TrueWalletSlip struct {
	Meta          ResponseMeta       `json:"-"`
	TransactionID string             `json:"transactionId"`
	Date          string             `json:"date"`
	Amount        float64            `json:"amount"`
	Sender        TrueWalletSender   `json:"sender"`
	Receiver      TrueWalletReceiver `json:"receiver"`
}

// MarshalJSON preserves absent fields on TrueWalletSlip.
func (v TrueWalletSlip) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// TrueWalletSender contains typed wire fields.
type TrueWalletSender struct {
	Meta ResponseMeta `json:"-"`
	Name string       `json:"name"`
}

// MarshalJSON preserves absent fields on TrueWalletSender.
func (v TrueWalletSender) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// TrueWalletReceiver contains typed wire fields.
type TrueWalletReceiver struct {
	Meta  ResponseMeta `json:"-"`
	Name  string       `json:"name"`
	Phone string       `json:"phone"`
}

// MarshalJSON preserves absent fields on TrueWalletReceiver.
func (v TrueWalletReceiver) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// Bank contains typed wire fields.
type Bank struct {
	Meta        ResponseMeta         `json:"-"`
	Code        string               `json:"code"`
	NameTh      string               `json:"nameTh"`
	NameEn      string               `json:"nameEn"`
	ShortCode   string               `json:"shortCode"`
	SwiftCode   string               `json:"swiftCode"`
	ExtraVerify *[]ExtraVerifyOption `json:"extraVerify"`
}

// MarshalJSON preserves absent fields on Bank.
func (v Bank) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// ExtraVerifyOption contains typed wire fields.
type ExtraVerifyOption struct {
	Meta  ResponseMeta `json:"-"`
	Value string       `json:"value"`
	Label string       `json:"label"`
}

// MarshalJSON preserves absent fields on ExtraVerifyOption.
func (v ExtraVerifyOption) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BankAccount contains typed wire fields.
type BankAccount struct {
	Meta        ResponseMeta `json:"-"`
	ID          int          `json:"id"`
	BankCode    string       `json:"bankCode"`
	BankNumber  string       `json:"bankNumber"`
	NameTh      string       `json:"nameTh"`
	NameEn      string       `json:"nameEn"`
	Type        string       `json:"type"`
	ExtraVerify *string      `json:"extraVerify"`
	MatchMode   *string      `json:"matchMode"`
	CreatedAt   string       `json:"createdAt"`
	UpdatedAt   string       `json:"updatedAt"`
	DeletedAt   *string      `json:"deletedAt"`
}

// MarshalJSON preserves absent fields on BankAccount.
func (v BankAccount) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BankAccountPage contains typed wire fields.
type BankAccountPage struct {
	Meta  ResponseMeta  `json:"-"`
	Items []BankAccount `json:"items"`
	Total int           `json:"total"`
	Page  int           `json:"page"`
	Limit int           `json:"limit"`
}

// MarshalJSON preserves absent fields on BankAccountPage.
func (v BankAccountPage) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// LinkResult contains typed wire fields.
type LinkResult struct {
	Meta   ResponseMeta `json:"-"`
	Linked bool         `json:"linked"`
}

// MarshalJSON preserves absent fields on LinkResult.
func (v LinkResult) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// UnlinkResult contains typed wire fields.
type UnlinkResult struct {
	Meta     ResponseMeta `json:"-"`
	Unlinked bool         `json:"unlinked"`
}

// MarshalJSON preserves absent fields on UnlinkResult.
func (v UnlinkResult) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// JobSubmission contains typed wire fields.
type JobSubmission struct {
	Meta  ResponseMeta `json:"-"`
	JobID string       `json:"jobId"`
}

// MarshalJSON preserves absent fields on JobSubmission.
func (v JobSubmission) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BatchSubmission contains typed wire fields.
type BatchSubmission struct {
	Meta    ResponseMeta `json:"-"`
	BatchID string       `json:"batchId"`
	Jobs    []BatchJob   `json:"jobs"`
}

// MarshalJSON preserves absent fields on BatchSubmission.
func (v BatchSubmission) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BatchJob contains typed wire fields.
type BatchJob struct {
	Meta  ResponseMeta `json:"-"`
	JobID string       `json:"jobId"`
	Index int          `json:"index"`
}

// MarshalJSON preserves absent fields on BatchJob.
func (v BatchJob) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// Job contains typed wire fields.
type Job struct {
	Meta             ResponseMeta            `json:"-"`
	JobID            string                  `json:"jobId"`
	BatchID          Field[string]           `json:"batchId"`
	Status           string                  `json:"status"`
	Result           Field[JobResult]        `json:"result"`
	Error            Field[JobError]         `json:"error"`
	Attempts         int                     `json:"attempts"`
	CreatedAt        string                  `json:"createdAt"`
	UpdatedAt        string                  `json:"updatedAt"`
	BranchID         Field[int]              `json:"branchId"`
	ServiceID        Field[int]              `json:"serviceId"`
	CallbackEventID  Field[string]           `json:"callbackEventId"`
	CallbackProtocol Field[int]              `json:"callbackProtocol"`
	CallbackDelivery Field[CallbackDelivery] `json:"callbackDelivery"`
}

// MarshalJSON preserves absent fields on Job.
func (v Job) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// JobResult contains typed wire fields.
type JobResult struct {
	Meta   ResponseMeta    `json:"-"`
	Status string          `json:"status"`
	Data   *BankSlipResult `json:"data"`
}

// MarshalJSON preserves absent fields on JobResult.
func (v JobResult) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// JobError contains typed wire fields.
type JobError struct {
	Meta    ResponseMeta  `json:"-"`
	Code    Field[string] `json:"code"`
	Message Field[string] `json:"message"`
	Webhook Field[string] `json:"webhook"`
	URL     Field[string] `json:"url"`
	Status  Field[int]    `json:"status"`
	At      Field[string] `json:"at"`
}

// MarshalJSON preserves absent fields on JobError.
func (v JobError) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// CallbackDelivery contains typed wire fields.
type CallbackDelivery struct {
	Meta       ResponseMeta `json:"-"`
	State      *string      `json:"state"`
	HTTPStatus *int         `json:"httpStatus"`
}

// MarshalJSON preserves absent fields on CallbackDelivery.
func (v CallbackDelivery) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// WebhookEvent contains typed wire fields.
type WebhookEvent struct {
	Meta      ResponseMeta        `json:"-"`
	JobID     string              `json:"jobId"`
	BatchID   *string             `json:"batchId"`
	Status    string              `json:"status"`
	Data      *BankSlipResult     `json:"data"`
	Error     Field[WebhookError] `json:"error"`
	Timestamp string              `json:"timestamp"`
}

// MarshalJSON preserves absent fields on WebhookEvent.
func (v WebhookEvent) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// WebhookError contains typed wire fields.
type WebhookError struct {
	Meta    ResponseMeta `json:"-"`
	Code    string       `json:"code"`
	Message string       `json:"message"`
}

// MarshalJSON preserves absent fields on WebhookError.
func (v WebhookError) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// V1Me contains typed wire fields.
type V1Me struct {
	Meta           ResponseMeta `json:"-"`
	Application    string       `json:"application"`
	UsedQuota      int          `json:"usedQuota"`
	MaxQuota       *int         `json:"maxQuota"`
	RemainingQuota *int         `json:"remainingQuota"`
	ExpiredAt      *string      `json:"expiredAt"`
	CurrentCredit  float64      `json:"currentCredit"`
}

// MarshalJSON preserves absent fields on V1Me.
func (v V1Me) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// QRCode contains typed wire fields.
type QRCode struct {
	Meta    ResponseMeta `json:"-"`
	Image   string       `json:"image"`
	Mime    string       `json:"mime"`
	Payload string       `json:"payload"`
}

// MarshalJSON preserves absent fields on QRCode.
func (v QRCode) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// ImageBytes decodes the PNG image.
func (v QRCode) ImageBytes() ([]byte, error) { return base64.StdEncoding.DecodeString(v.Image) }

// VerifyOptions contains typed wire fields.
type VerifyOptions struct {
	Meta           ResponseMeta   `json:"-"`
	Remark         Field[string]  `json:"remark"`
	MatchAccount   Field[bool]    `json:"matchAccount"`
	MatchAmount    Field[float64] `json:"matchAmount"`
	CheckDuplicate Field[bool]    `json:"checkDuplicate"`
}

// MarshalJSON preserves absent fields on VerifyOptions.
func (v VerifyOptions) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// CreateBankAccountParams contains typed wire fields.
type CreateBankAccountParams struct {
	Meta        ResponseMeta  `json:"-"`
	BankCode    string        `json:"bankCode"`
	BankNumber  string        `json:"bankNumber"`
	NameTh      string        `json:"nameTh"`
	NameEn      string        `json:"nameEn"`
	Type        AccountType   `json:"type"`
	ExtraVerify Field[string] `json:"extraVerify"`
	MatchMode   Field[string] `json:"matchMode"`
}

// MarshalJSON preserves absent fields on CreateBankAccountParams.
func (v CreateBankAccountParams) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// UpdateBankAccountParams contains typed wire fields.
type UpdateBankAccountParams struct {
	Meta        ResponseMeta       `json:"-"`
	BankCode    Field[string]      `json:"bankCode"`
	BankNumber  Field[string]      `json:"bankNumber"`
	NameTh      Field[string]      `json:"nameTh"`
	NameEn      Field[string]      `json:"nameEn"`
	Type        Field[AccountType] `json:"type"`
	ExtraVerify Field[*string]     `json:"extraVerify"`
	MatchMode   Field[string]      `json:"matchMode"`
}

// MarshalJSON preserves absent fields on UpdateBankAccountParams.
func (v UpdateBankAccountParams) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BatchSlip contains typed wire fields.
type BatchSlip struct {
	Meta           ResponseMeta   `json:"-"`
	Remark         Field[string]  `json:"remark"`
	MatchAccount   Field[bool]    `json:"matchAccount"`
	MatchAmount    Field[float64] `json:"matchAmount"`
	CheckDuplicate Field[bool]    `json:"checkDuplicate"`
	Payload        string         `json:"payload"`
}

// MarshalJSON preserves absent fields on BatchSlip.
func (v BatchSlip) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// AsyncVerifyOptions contains typed wire fields.
type AsyncVerifyOptions struct {
	Meta           ResponseMeta   `json:"-"`
	Remark         Field[string]  `json:"remark"`
	MatchAccount   Field[bool]    `json:"matchAccount"`
	MatchAmount    Field[float64] `json:"matchAmount"`
	CheckDuplicate Field[bool]    `json:"checkDuplicate"`
	CallbackURL    Field[string]  `json:"callbackUrl"`
}

// MarshalJSON preserves absent fields on AsyncVerifyOptions.
func (v AsyncVerifyOptions) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BatchOptions contains typed wire fields.
type BatchOptions struct {
	Meta        ResponseMeta  `json:"-"`
	CallbackURL Field[string] `json:"callbackUrl"`
}

// MarshalJSON preserves absent fields on BatchOptions.
func (v BatchOptions) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// V1VerifyOptions contains typed wire fields.
type V1VerifyOptions struct {
	Meta           ResponseMeta `json:"-"`
	CheckDuplicate Field[bool]  `json:"checkDuplicate"`
}

// MarshalJSON preserves absent fields on V1VerifyOptions.
func (v V1VerifyOptions) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// ListBankAccountsParams contains typed wire fields.
type ListBankAccountsParams struct {
	Meta     ResponseMeta       `json:"-"`
	Type     Field[AccountType] `json:"type"`
	BankCode Field[string]      `json:"bankCode"`
	Page     Field[int]         `json:"page"`
	Limit    Field[int]         `json:"limit"`
}

// MarshalJSON preserves absent fields on ListBankAccountsParams.
func (v ListBankAccountsParams) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// QRRequest contains typed wire fields.
//
// One flat struct for every Type; the API checks the combinations. With BillerID, Ref1 is
// optional and Ref2 needs Ref1. KSHOP Ref1 is the shop id KB + 12 digits, MAE_MANEE Ref1 the
// 15-digit shop id. TUNGNGERN needs MerchantName, the English shop name. Fields are sent as given.
type QRRequest struct {
	Meta         ResponseMeta   `json:"-"`
	Type         QRType         `json:"type"`
	Msisdn       Field[string]  `json:"msisdn"`
	NatID        Field[string]  `json:"natId"`
	EWalletID    Field[string]  `json:"eWalletId"`
	BillerID     Field[string]  `json:"billerId"`
	Ref1         Field[string]  `json:"ref1"`
	Ref2         Field[string]  `json:"ref2"`
	Amount       Field[float64] `json:"amount"`
	MerchantName Field[string]  `json:"merchantName"`
}

// MarshalJSON preserves absent fields on QRRequest.
func (v QRRequest) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// RateLimit contains typed wire fields.
type RateLimit struct {
	Meta      ResponseMeta `json:"-"`
	Limit     int          `json:"limit"`
	Remaining int          `json:"remaining"`
	Reset     int          `json:"reset"`
}

// MarshalJSON preserves absent fields on RateLimit.
func (v RateLimit) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// ResponseMeta contains typed wire fields.
type ResponseMeta struct {
	Status    int               `json:"status"`
	RequestID *string           `json:"requestId"`
	Message   *string           `json:"message"`
	RateLimit *RateLimit        `json:"rateLimit"`
	Headers   map[string]string `json:"headers"`
}

// MarshalJSON preserves absent fields on ResponseMeta.
func (v ResponseMeta) MarshalJSON() ([]byte, error) { return marshalFields(v) }

// BankList wraps the bank array with response metadata.
type BankList struct {
	Items []Bank
	Meta  ResponseMeta
}

// MarshalJSON returns the plain bank array.
func (v BankList) MarshalJSON() ([]byte, error) { return json.Marshal(v.Items) }

// UnmarshalJSON reads the plain bank array.
func (v *BankList) UnmarshalJSON(b []byte) error { return json.Unmarshal(b, &v.Items) }
