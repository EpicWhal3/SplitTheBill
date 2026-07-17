package calculation

import (
	"strings"
	"testing"

	"splitthebill/backend/internal/domain"
)

func TestCalculateSimpleBill(t *testing.T) {
	input := BillInput{
		Participants: []domain.Participant{
			{
				ID:   "p1",
				Name: "Аня",
			},
			{
				ID:   "p2",
				Name: "Борис",
			},
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

	if got, want := sumResults(results), int64(3180); got != want {
		t.Fatalf(
			"expected total %d, got %d",
			want,
			got,
		)
	}
}

func TestCalculateSplitsRemainderWithoutLosingMoney(
	t *testing.T,
) {
	input := BillInput{
		Participants: []domain.Participant{
			{ID: "p1", Name: "A"},
			{ID: "p2", Name: "B"},
			{ID: "p3", Name: "C"},
		},
		Items: []domain.ReceiptItem{
			{
				ID:    "i1",
				Name:  "Shared",
				Total: 100,
			},
		},
		Assignments: []domain.ItemAssignment{
			{
				ItemID:        "i1",
				ParticipantID: "p1",
				Weight:        1,
			},
			{
				ItemID:        "i1",
				ParticipantID: "p2",
				Weight:        1,
			},
			{
				ItemID:        "i1",
				ParticipantID: "p3",
				Weight:        1,
			},
		},
	}

	results, err := Calculate(input)
	if err != nil {
		t.Fatal(err)
	}

	if got := sumResults(results); got != 100 {
		t.Fatalf(
			"expected 100, got %d",
			got,
		)
	}

	amounts := map[string]int64{}

	for _, result := range results {
		amounts[result.ParticipantID] =
			result.TotalAmount
	}

	if amounts["p1"] != 34 ||
		amounts["p2"] != 33 ||
		amounts["p3"] != 33 {
		t.Fatalf(
			"unexpected deterministic split: %#v",
			amounts,
		)
	}
}

func TestCalculateWeightedSplit(
	t *testing.T,
) {
	input := BillInput{
		Participants: []domain.Participant{
			{ID: "p1", Name: "A"},
			{ID: "p2", Name: "B"},
		},
		Items: []domain.ReceiptItem{
			{
				ID:    "i1",
				Name:  "Pizza",
				Total: 900,
			},
		},
		Assignments: []domain.ItemAssignment{
			{
				ItemID:        "i1",
				ParticipantID: "p1",
				Weight:        2,
			},
			{
				ItemID:        "i1",
				ParticipantID: "p2",
				Weight:        1,
			},
		},
	}

	results, err := Calculate(input)
	if err != nil {
		t.Fatal(err)
	}

	if results[0].TotalAmount != 600 ||
		results[1].TotalAmount != 300 {
		t.Fatalf(
			"unexpected weighted split: %#v",
			results,
		)
	}
}

func TestCalculateRejectsUnassignedItem(
	t *testing.T,
) {
	_, err := Calculate(
		BillInput{
			Participants: []domain.Participant{
				{ID: "p1", Name: "A"},
			},
			Items: []domain.ReceiptItem{
				{
					ID:    "i1",
					Name:  "Pizza",
					Total: 900,
				},
			},
		},
	)

	assertErrorContains(
		t,
		err,
		"item has no assignments",
	)
}

func TestCalculateRejectsUnknownParticipant(
	t *testing.T,
) {
	_, err := Calculate(
		BillInput{
			Participants: []domain.Participant{
				{ID: "p1", Name: "A"},
			},
			Items: []domain.ReceiptItem{
				{
					ID:    "i1",
					Name:  "Pizza",
					Total: 900,
				},
			},
			Assignments: []domain.ItemAssignment{
				{
					ItemID:        "i1",
					ParticipantID: "missing",
					Weight:        1,
				},
			},
		},
	)

	assertErrorContains(
		t,
		err,
		"unknown participant",
	)
}

func TestCalculateRejectsDuplicateAssignment(
	t *testing.T,
) {
	_, err := Calculate(
		BillInput{
			Participants: []domain.Participant{
				{ID: "p1", Name: "A"},
			},
			Items: []domain.ReceiptItem{
				{
					ID:    "i1",
					Name:  "Pizza",
					Total: 900,
				},
			},
			Assignments: []domain.ItemAssignment{
				{
					ItemID:        "i1",
					ParticipantID: "p1",
					Weight:        1,
				},
				{
					ItemID:        "i1",
					ParticipantID: "p1",
					Weight:        2,
				},
			},
		},
	)

	assertErrorContains(
		t,
		err,
		"duplicate assignment",
	)
}

func TestCalculateRejectsDiscountLargerThanBill(
	t *testing.T,
) {
	_, err := Calculate(
		BillInput{
			Participants: []domain.Participant{
				{ID: "p1", Name: "A"},
			},
			Items: []domain.ReceiptItem{
				{
					ID:    "i1",
					Name:  "Pizza",
					Total: 900,
				},
			},
			Assignments: []domain.ItemAssignment{
				{
					ItemID:        "i1",
					ParticipantID: "p1",
					Weight:        1,
				},
			},
			Discount: 901,
		},
	)

	assertErrorContains(
		t,
		err,
		"discount exceeds bill total",
	)
}

func TestCalculateAllowsFullDiscount(
	t *testing.T,
) {
	results, err := Calculate(
		BillInput{
			Participants: []domain.Participant{
				{ID: "p1", Name: "A"},
			},
			Items: []domain.ReceiptItem{
				{
					ID:    "i1",
					Name:  "Pizza",
					Total: 900,
				},
			},
			Assignments: []domain.ItemAssignment{
				{
					ItemID:        "i1",
					ParticipantID: "p1",
					Weight:        1,
				},
			},
			Discount: 900,
		},
	)

	if err != nil {
		t.Fatal(err)
	}

	if results[0].TotalAmount != 0 {
		t.Fatalf(
			"expected zero total, got %d",
			results[0].TotalAmount,
		)
	}
}

func sumResults(
	results []domain.ParticipantResult,
) int64 {
	var total int64

	for _, result := range results {
		total += result.TotalAmount
	}

	return total
}

func assertErrorContains(
	t *testing.T,
	err error,
	part string,
) {
	t.Helper()

	if err == nil {
		t.Fatalf(
			"expected error containing %q",
			part,
		)
	}

	if !strings.Contains(
		err.Error(),
		part,
	) {
		t.Fatalf(
			"expected error containing %q, got %q",
			part,
			err.Error(),
		)
	}
}
