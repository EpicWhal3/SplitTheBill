package domain

type Room struct {
	ID          string
	Title       string
	Currency    string
	Subtotal    int64
	ServiceFee  int64
	TipAmount   int64
	Discount    int64
	TotalAmount int64
}

type Participant struct {
	ID     string
	RoomID string
	Name   string
}

type ReceiptItem struct {
	ID        string
	RoomID    string
	Name      string
	Quantity  int
	UnitPrice int64
	Total     int64
}

type ItemAssignment struct {
	ItemID        string
	ParticipantID string
	Weight        int64
}

type ParticipantResult struct {
	ParticipantID string
	Name          string
	BaseAmount    int64
	ServiceShare  int64
	TipShare      int64
	DiscountShare int64
	TotalAmount   int64
}
