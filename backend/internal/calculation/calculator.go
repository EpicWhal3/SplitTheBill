package calculation

import (
	"errors"
	"fmt"
	"sort"

	"splitthebill/backend/internal/domain"
)

type BillInput struct {
	Participants []domain.Participant
	Items        []domain.ReceiptItem
	Assignments  []domain.ItemAssignment

	ServiceFee int64
	TipAmount  int64
	Discount   int64
}

func Calculate(
	input BillInput,
) ([]domain.ParticipantResult, error) {
	if len(input.Participants) == 0 {
		return nil, errors.New("no participants")
	}

	if len(input.Items) == 0 {
		return nil, errors.New("no receipt items")
	}

	if input.ServiceFee < 0 ||
		input.TipAmount < 0 ||
		input.Discount < 0 {
		return nil, errors.New(
			"service fee, tip amount and discount must be non-negative",
		)
	}

	participantsByID := make(
		map[string]domain.Participant,
		len(input.Participants),
	)

	base := make(
		map[string]int64,
		len(input.Participants),
	)

	for _, participant := range input.Participants {
		if participant.ID == "" {
			return nil, errors.New("participant id is required")
		}

		if _, exists := participantsByID[participant.ID]; exists {
			return nil, fmt.Errorf(
				"duplicate participant: %s",
				participant.ID,
			)
		}

		participantsByID[participant.ID] = participant
		base[participant.ID] = 0
	}

	itemsByID := make(
		map[string]domain.ReceiptItem,
		len(input.Items),
	)

	for _, item := range input.Items {
		if item.ID == "" {
			return nil, errors.New("item id is required")
		}

		if item.Total <= 0 {
			return nil, fmt.Errorf(
				"item total must be positive: %s",
				item.Name,
			)
		}

		if _, exists := itemsByID[item.ID]; exists {
			return nil, fmt.Errorf(
				"duplicate item: %s",
				item.ID,
			)
		}

		itemsByID[item.ID] = item
	}

	assignmentsByItem := make(
		map[string][]domain.ItemAssignment,
	)

	seenAssignments := make(
		map[string]struct{},
		len(input.Assignments),
	)

	for _, assignment := range input.Assignments {
		if assignment.Weight <= 0 {
			return nil, errors.New(
				"assignment weight must be positive",
			)
		}

		if _, exists := itemsByID[assignment.ItemID]; !exists {
			return nil, fmt.Errorf(
				"assignment references unknown item: %s",
				assignment.ItemID,
			)
		}

		if _, exists := participantsByID[assignment.ParticipantID]; !exists {
			return nil, fmt.Errorf(
				"assignment references unknown participant: %s",
				assignment.ParticipantID,
			)
		}

		key := assignment.ItemID +
			"\x00" +
			assignment.ParticipantID

		if _, exists := seenAssignments[key]; exists {
			return nil, fmt.Errorf(
				"duplicate assignment for item %s and participant %s",
				assignment.ItemID,
				assignment.ParticipantID,
			)
		}

		seenAssignments[key] = struct{}{}

		assignmentsByItem[assignment.ItemID] = append(
			assignmentsByItem[assignment.ItemID],
			assignment,
		)
	}

	for _, item := range input.Items {
		assignments := assignmentsByItem[item.ID]

		if len(assignments) == 0 {
			return nil, errors.New(
				"item has no assignments: " + item.Name,
			)
		}

		var totalWeight int64

		for _, assignment := range assignments {
			totalWeight += assignment.Weight
		}

		shares := splitByWeights(
			item.Total,
			assignments,
			totalWeight,
		)

		for participantID, amount := range shares {
			base[participantID] += amount
		}
	}

	var subtotal int64

	for _, amount := range base {
		subtotal += amount
	}

	if subtotal <= 0 {
		return nil, errors.New("subtotal must be positive")
	}

	maximumDiscount :=
		subtotal +
			input.ServiceFee +
			input.TipAmount

	if input.Discount > maximumDiscount {
		return nil, fmt.Errorf(
			"discount exceeds bill total: maximum is %d",
			maximumDiscount,
		)
	}

	serviceShares := splitProportionally(
		input.ServiceFee,
		base,
	)

	tipShares := splitProportionally(
		input.TipAmount,
		base,
	)

	discountShares := splitProportionally(
		input.Discount,
		base,
	)

	results := make(
		[]domain.ParticipantResult,
		0,
		len(input.Participants),
	)

	for _, participant := range input.Participants {
		result := domain.ParticipantResult{
			ParticipantID: participant.ID,
			Name:          participant.Name,
			BaseAmount:    base[participant.ID],
			ServiceShare:  serviceShares[participant.ID],
			TipShare:      tipShares[participant.ID],
			DiscountShare: discountShares[participant.ID],
		}

		result.TotalAmount =
			result.BaseAmount +
				result.ServiceShare +
				result.TipShare -
				result.DiscountShare

		if result.TotalAmount < 0 {
			return nil, fmt.Errorf(
				"negative participant total for %s",
				participant.Name,
			)
		}

		results = append(results, result)
	}

	return results, nil
}

func splitByWeights(
	total int64,
	assignments []domain.ItemAssignment,
	totalWeight int64,
) map[string]int64 {
	result := make(map[string]int64)

	remainders := make(
		[]remainder,
		0,
		len(assignments),
	)

	var distributed int64

	for _, assignment := range assignments {
		numerator := total * assignment.Weight
		amount := numerator / totalWeight
		rem := numerator % totalWeight

		result[assignment.ParticipantID] += amount
		distributed += amount

		remainders = append(
			remainders,
			remainder{
				ParticipantID: assignment.ParticipantID,
				Value:         rem,
			},
		)
	}

	left := total - distributed

	sort.Slice(
		remainders,
		func(i, j int) bool {
			if remainders[i].Value == remainders[j].Value {
				return remainders[i].ParticipantID <
					remainders[j].ParticipantID
			}

			return remainders[i].Value >
				remainders[j].Value
		},
	)

	for i := int64(0); i < left; i++ {
		index := i % int64(len(remainders))

		result[remainders[index].ParticipantID]++
	}

	return result
}

func splitProportionally(
	total int64,
	base map[string]int64,
) map[string]int64 {
	result := make(
		map[string]int64,
		len(base),
	)

	for participantID := range base {
		result[participantID] = 0
	}

	if total == 0 {
		return result
	}

	var baseTotal int64

	for _, amount := range base {
		baseTotal += amount
	}

	if baseTotal <= 0 {
		return result
	}

	remainders := make(
		[]remainder,
		0,
		len(base),
	)

	var distributed int64

	for participantID, amount := range base {
		numerator := total * amount
		share := numerator / baseTotal
		rem := numerator % baseTotal

		result[participantID] = share
		distributed += share

		remainders = append(
			remainders,
			remainder{
				ParticipantID: participantID,
				Value:         rem,
			},
		)
	}

	left := total - distributed

	sort.Slice(
		remainders,
		func(i, j int) bool {
			if remainders[i].Value == remainders[j].Value {
				return remainders[i].ParticipantID <
					remainders[j].ParticipantID
			}

			return remainders[i].Value >
				remainders[j].Value
		},
	)

	for i := int64(0); i < left; i++ {
		index := i % int64(len(remainders))

		result[remainders[index].ParticipantID]++
	}

	return result
}

type remainder struct {
	ParticipantID string
	Value         int64
}
