package calculation

import (
	"testing"

	"splitthebill/backend/internal/domain"
)

func TestCalculateSimpleBill(t *testing.T) {
	input := BillInput{
		Participants: []domain.Participant{
			{ID: "p1", Name: "Аня"},
			{ID: "p2", Name: "Борис"},
		},
		Items: []domain.ReceiptItem{
			{
				ID:    "i1",
				Name:  "Burger",
				Total: 1200,
			},
			{
				ID:    "i2",
				Name:  "Pizza",
				Total: 1600,
			},
		},
		Assignments: []domain.ItemAssignment{
			{
				ItemID:        "i1",
				ParticipantID: "p1",
				Weight:        1,
			},
			{
				ItemID:        "i2",
				ParticipantID: "p1",
				Weight:        1,
			},
			{
				ItemID:        "i2",
				ParticipantID: "p2",
				Weight:        1,
			},
		},
		ServiceFee: 280,
		TipAmount:  200,
		Discount:   100,
	}

	results, err := Calculate(input)
	if err != nil {
		t.Fatal(err)
	}

	var total int64

	for _, result := range results {
		total += result.TotalAmount
	}

	expected := int64(1200 + 1600 + 280 + 200 - 100)

	if total != expected {
		t.Fatalf("expected total %d, got %d", expected, total)
	}
}
