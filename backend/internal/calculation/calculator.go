package calculation

import (
	"errors"
	"sort"
	"splitcheck/backend/internal/domain"
)

type BillInput struct {
	Participants []domain.Participant
	Items        []domain.ReceiptItem
	Assignments  []domain.ItemAssignment

	ServiceFee int64
	TipAmount  int64
	Discount   int64
}

func Calculate(input BillInput) ([]domain.ParticipantResult, error) {
	if len(input.Participants) == 0 {
		return nil, errors.New("no participants")
	}

	base := map[string]int64{}

	for _, p := range input.Participants {
		base[p.ID] = 0
	}

	assignmentsByItem := map[string][]domain.ItemAssignment{}

	for _, assignment := range input.Assignments {
		if assignment.Weight <= 0 {
			return nil, errors.New("assignment weight must be positive")
		}

		assignmentsByItem[assignment.ItemID] = append(
			assignmentsByItem[assignment.ItemID],
			assignment,
		)
	}

	for _, item := range input.Items {
		assignments := assignmentsByItem[item.ID]

		if len(assignments) == 0 {
			return nil, errors.New("item has no assignments: " + item.Name)
		}

		var totalWeight int64

		for _, assignment := range assignments {
			totalWeight += assignment.Weight
		}

		shares := splitByWeights(item.Total, assignments, totalWeight)

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

	serviceShares := splitProportionally(input.ServiceFee, base)
	tipShares := splitProportionally(input.TipAmount, base)
	discountShares := splitProportionally(input.Discount, base)

	results := make([]domain.ParticipantResult, 0, len(input.Participants))

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
	remainders := make([]remainder, 0, len(assignments))

	var distributed int64

	for _, assignment := range assignments {
		numerator := total * assignment.Weight
		amount := numerator / totalWeight
		rem := numerator % totalWeight

		result[assignment.ParticipantID] += amount
		distributed += amount

		remainders = append(remainders, remainder{
			ParticipantID: assignment.ParticipantID,
			Value:         rem,
		})
	}

	left := total - distributed

	sort.Slice(remainders, func(i, j int) bool {
		return remainders[i].Value > remainders[j].Value
	})

	for i := range left {
		index := i % int64(len(remainders))
		result[remainders[index].ParticipantID]++
	}

	return result
}

func splitProportionally(total int64, base map[string]int64) map[string]int64 {
	result := map[string]int64{}

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

	type participantRemainder struct {
		ParticipantID string
		Value         int64
	}

	remainders := make([]participantRemainder, 0, len(base))

	var distributed int64

	for participantID, amount := range base {
		numerator := total * amount
		share := numerator / baseTotal
		rem := numerator % baseTotal

		result[participantID] = share
		distributed += share

		remainders = append(remainders, participantRemainder{
			ParticipantID: participantID,
			Value:         rem,
		})
	}

	left := total - distributed

	sort.Slice(remainders, func(i, j int) bool {
		return remainders[i].Value > remainders[j].Value
	})

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
