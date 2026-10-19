package store

import (
	"context"
	"database/sql"
	"errors"

	"splitthebill/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	db *pgxpool.Pool
}

func NewPostgresStore(
	ctx context.Context,
	databaseURL string,
) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostgresStore{db: pool}, nil
}

func (s *PostgresStore) Close() {
	s.db.Close()
}

func (s *PostgresStore) CreateRoom(
	room domain.Room,
) (domain.Room, error) {
	ctx := context.Background()
	room.ID = newID()

	if room.AdminToken == "" {
		room.AdminToken = newToken()
	}
	if room.DiscountMode == "" {
		room.DiscountMode = domain.DiscountModeProportional
	}
	if room.Status == "" {
		room.Status = domain.RoomStatusDraft
	}

	query := `
		INSERT INTO rooms (
			id,
			title,
			currency,
			service_fee,
			tip_amount,
			discount,
			discount_mode,
			expected_total,
			payer_participant_id,
			status,
			finalized_at,
			admin_token
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9, ''), $10, $11, $12)
	`

	_, err := s.db.Exec(
		ctx,
		query,
		room.ID,
		room.Title,
		room.Currency,
		room.ServiceFee,
		room.TipAmount,
		room.Discount,
		room.DiscountMode,
		room.ExpectedTotal,
		room.PayerParticipantID,
		room.Status,
		room.FinalizedAt,
		room.AdminToken,
	)
	if err != nil {
		return domain.Room{}, err
	}

	return room, nil
}

func (s *PostgresStore) GetRoom(
	roomID string,
) (domain.Room, error) {
	ctx := context.Background()

	query := `
		SELECT
			id,
			title,
			currency,
			service_fee,
			tip_amount,
			discount,
			discount_mode,
			expected_total,
			COALESCE(payer_participant_id, ''),
			status,
			finalized_at,
			admin_token
		FROM rooms
		WHERE id = $1
	`

	var room domain.Room
	var finalizedAt sql.NullTime

	err := s.db.QueryRow(
		ctx,
		query,
		roomID,
	).Scan(
		&room.ID,
		&room.Title,
		&room.Currency,
		&room.ServiceFee,
		&room.TipAmount,
		&room.Discount,
		&room.DiscountMode,
		&room.ExpectedTotal,
		&room.PayerParticipantID,
		&room.Status,
		&finalizedAt,
		&room.AdminToken,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Room{}, ErrorNotFound
	}
	if err != nil {
		return domain.Room{}, err
	}
	if finalizedAt.Valid {
		room.FinalizedAt = &finalizedAt.Time
	}

	return room, nil
}

func (s *PostgresStore) UpdateRoom(
	room domain.Room,
) (domain.Room, error) {
	ctx := context.Background()

	query := `
		UPDATE rooms SET
			title = $2,
			currency = $3,
			service_fee = $4,
			tip_amount = $5,
			discount = $6,
			discount_mode = $7,
			expected_total = $8,
			payer_participant_id = NULLIF($9, ''),
			status = $10,
			finalized_at = $11,
			updated_at = now()
		WHERE id = $1
	`

	commandTag, err := s.db.Exec(
		ctx,
		query,
		room.ID,
		room.Title,
		room.Currency,
		room.ServiceFee,
		room.TipAmount,
		room.Discount,
		room.DiscountMode,
		room.ExpectedTotal,
		room.PayerParticipantID,
		room.Status,
		room.FinalizedAt,
	)
	if err != nil {
		return domain.Room{}, err
	}
	if commandTag.RowsAffected() == 0 {
		return domain.Room{}, ErrorNotFound
	}

	return room, nil
}

func (s *PostgresStore) AddParticipant(
	roomID string,
	participant domain.Participant,
) (domain.Participant, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return domain.Participant{}, err
	}

	if err := s.ensureParticipantNameAvailable(
		ctx,
		roomID,
		participant.Name,
		"",
	); err != nil {
		return domain.Participant{}, err
	}

	participant.ID = newID()
	participant.RoomID = roomID
	participant.Claimed = false
	if participant.AccessToken == "" {
		participant.AccessToken = newParticipantPIN()
	}

	query := `
		INSERT INTO participants (
			id,
			room_id,
			name,
			access_token,
			claimed
		)
		VALUES ($1, $2, $3, $4, $5)
	`

	_, err := s.db.Exec(
		ctx,
		query,
		participant.ID,
		participant.RoomID,
		participant.Name,
		participant.AccessToken,
		participant.Claimed,
	)
	if err != nil {
		return domain.Participant{}, err
	}

	return participant, nil
}

func (s *PostgresStore) JoinParticipantByToken(
	roomID string,
	token string,
) (domain.Participant, error) {
	ctx := context.Background()

	if token == "" {
		return domain.Participant{}, ErrorParticipantNotFound
	}

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return domain.Participant{}, err
	}

	var participant domain.Participant

	err := s.db.QueryRow(
		ctx,
		`
			UPDATE participants
			SET claimed = true
			WHERE room_id = $1 AND access_token = $2
			RETURNING
				id,
				room_id,
				name,
				access_token,
				claimed
		`,
		roomID,
		token,
	).Scan(
		&participant.ID,
		&participant.RoomID,
		&participant.Name,
		&participant.AccessToken,
		&participant.Claimed,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Participant{}, ErrorParticipantNotFound
	}

	if err != nil {
		return domain.Participant{}, err
	}

	return participant, nil
}

func (s *PostgresStore) RotateParticipantToken(
	roomID string,
	participantID string,
) (domain.Participant, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return domain.Participant{}, err
	}

	var participant domain.Participant

	err := s.db.QueryRow(
		ctx,
		`
			UPDATE participants
			SET access_token = $3
			WHERE room_id = $1 AND id = $2
			RETURNING
				id,
				room_id,
				name,
				access_token,
				claimed
		`,
		roomID,
		participantID,
		newParticipantPIN(),
	).Scan(
		&participant.ID,
		&participant.RoomID,
		&participant.Name,
		&participant.AccessToken,
		&participant.Claimed,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Participant{}, ErrorParticipantNotFound
	}

	if err != nil {
		return domain.Participant{}, err
	}

	return participant, nil
}

func (s *PostgresStore) FindParticipantByToken(
	roomID string,
	token string,
) (domain.Participant, error) {
	ctx := context.Background()
	var participant domain.Participant

	err := s.db.QueryRow(
		ctx,
		`
			SELECT
				id,
				room_id,
				name,
				access_token,
				claimed
			FROM participants
			WHERE room_id = $1 AND access_token = $2
		`,
		roomID,
		token,
	).Scan(
		&participant.ID,
		&participant.RoomID,
		&participant.Name,
		&participant.AccessToken,
		&participant.Claimed,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Participant{}, ErrorParticipantNotFound
	}

	if err != nil {
		return domain.Participant{}, err
	}

	return participant, nil
}

func (s *PostgresStore) ListParticipants(
	roomID string,
) ([]domain.Participant, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		ctx,
		`
			SELECT
				id,
				room_id,
				name,
				COALESCE(access_token, ''),
				claimed
			FROM participants
			WHERE room_id = $1
			ORDER BY created_at ASC
		`,
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	participants := make([]domain.Participant, 0)

	for rows.Next() {
		var participant domain.Participant

		if err := rows.Scan(
			&participant.ID,
			&participant.RoomID,
			&participant.Name,
			&participant.AccessToken,
			&participant.Claimed,
		); err != nil {
			return nil, err
		}

		participants = append(participants, participant)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return participants, nil
}

func (s *PostgresStore) UpdateParticipant(
	roomID string,
	participant domain.Participant,
) (domain.Participant, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return domain.Participant{}, err
	}

	if err := s.ensureParticipantNameAvailable(
		ctx,
		roomID,
		participant.Name,
		participant.ID,
	); err != nil {
		return domain.Participant{}, err
	}

	participant.RoomID = roomID

	commandTag, err := s.db.Exec(
		ctx,
		`
			UPDATE participants
			SET name = $3
			WHERE room_id = $1 AND id = $2
		`,
		roomID,
		participant.ID,
		participant.Name,
	)
	if err != nil {
		return domain.Participant{}, err
	}

	if commandTag.RowsAffected() == 0 {
		return domain.Participant{}, ErrorParticipantNotFound
	}

	return participant, nil
}

func (s *PostgresStore) DeleteParticipant(
	roomID string,
	participantID string,
) error {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return err
	}

	commandTag, err := s.db.Exec(
		ctx,
		`
			DELETE FROM participants
			WHERE room_id = $1 AND id = $2
		`,
		roomID,
		participantID,
	)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrorParticipantNotFound
	}

	return nil
}

func (s *PostgresStore) AddItem(
	roomID string,
	item domain.ReceiptItem,
) (domain.ReceiptItem, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return domain.ReceiptItem{}, err
	}

	item.ID = newID()
	item.RoomID = roomID

	query := `
		INSERT INTO receipt_items (
			id,
			room_id,
			name,
			quantity,
			unit_price,
			total
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := s.db.Exec(
		ctx,
		query,
		item.ID,
		item.RoomID,
		item.Name,
		item.Quantity,
		item.UnitPrice,
		item.Total,
	)
	if err != nil {
		return domain.ReceiptItem{}, err
	}

	return item, nil
}

func (s *PostgresStore) ListItems(
	roomID string,
) ([]domain.ReceiptItem, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		ctx,
		`
			SELECT
				id,
				room_id,
				name,
				quantity,
				unit_price,
				total
			FROM receipt_items
			WHERE room_id = $1
			ORDER BY created_at ASC
		`,
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.ReceiptItem, 0)

	for rows.Next() {
		var item domain.ReceiptItem

		if err := rows.Scan(
			&item.ID,
			&item.RoomID,
			&item.Name,
			&item.Quantity,
			&item.UnitPrice,
			&item.Total,
		); err != nil {
			return nil, err
		}

		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *PostgresStore) UpdateItem(
	roomID string,
	item domain.ReceiptItem,
) (domain.ReceiptItem, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return domain.ReceiptItem{}, err
	}

	item.RoomID = roomID

	commandTag, err := s.db.Exec(
		ctx,
		`
			UPDATE receipt_items SET
				name = $3,
				quantity = $4,
				unit_price = $5,
				total = $6
			WHERE room_id = $1 AND id = $2
		`,
		roomID,
		item.ID,
		item.Name,
		item.Quantity,
		item.UnitPrice,
		item.Total,
	)
	if err != nil {
		return domain.ReceiptItem{}, err
	}

	if commandTag.RowsAffected() == 0 {
		return domain.ReceiptItem{}, ErrorItemNotFound
	}

	return item, nil
}

func (s *PostgresStore) DeleteItem(
	roomID string,
	itemID string,
) error {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return err
	}

	commandTag, err := s.db.Exec(
		ctx,
		`
			DELETE FROM receipt_items
			WHERE room_id = $1 AND id = $2
		`,
		roomID,
		itemID,
	)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrorItemNotFound
	}

	return nil
}

func (s *PostgresStore) AddAssignment(
	roomID string,
	assignment domain.ItemAssignment,
) (domain.ItemAssignment, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return domain.ItemAssignment{}, err
	}

	if err := s.ensureItemExists(
		ctx,
		roomID,
		assignment.ItemID,
	); err != nil {
		return domain.ItemAssignment{}, ErrorItemNotFound
	}

	if err := s.ensureParticipantExists(
		ctx,
		roomID,
		assignment.ParticipantID,
	); err != nil {
		return domain.ItemAssignment{}, ErrorParticipantNotFound
	}

	query := `
		INSERT INTO item_assignments (
			room_id,
			item_id,
			participant_id,
			weight
		)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (
			room_id,
			item_id,
			participant_id
		)
		DO UPDATE SET
			weight = EXCLUDED.weight
	`

	_, err := s.db.Exec(
		ctx,
		query,
		roomID,
		assignment.ItemID,
		assignment.ParticipantID,
		assignment.Weight,
	)
	if err != nil {
		return domain.ItemAssignment{}, err
	}

	return assignment, nil
}

func (s *PostgresStore) ListAssignments(
	roomID string,
) ([]domain.ItemAssignment, error) {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		ctx,
		`
			SELECT
				item_id,
				participant_id,
				weight
			FROM item_assignments
			WHERE room_id = $1
			ORDER BY created_at ASC
		`,
		roomID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := make([]domain.ItemAssignment, 0)

	for rows.Next() {
		var assignment domain.ItemAssignment

		if err := rows.Scan(
			&assignment.ItemID,
			&assignment.ParticipantID,
			&assignment.Weight,
		); err != nil {
			return nil, err
		}

		assignments = append(assignments, assignment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return assignments, nil
}

func (s *PostgresStore) DeleteAssignment(
	roomID string,
	itemID string,
	participantID string,
) error {
	ctx := context.Background()

	if err := s.ensureRoomExists(ctx, roomID); err != nil {
		return err
	}

	commandTag, err := s.db.Exec(
		ctx,
		`
			DELETE FROM item_assignments
			WHERE
				room_id = $1
				AND item_id = $2
				AND participant_id = $3
		`,
		roomID,
		itemID,
		participantID,
	)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrorNotFound
	}

	return nil
}

func (s *PostgresStore) DeleteRoom(roomID string) error {
	ctx := context.Background()

	commandTag, err := s.db.Exec(
		ctx,
		`DELETE FROM rooms WHERE id = $1`,
		roomID,
	)
	if err != nil {
		return err
	}

	if commandTag.RowsAffected() == 0 {
		return ErrorNotFound
	}

	return nil
}

func (s *PostgresStore) RotateAdminToken(
	roomID string,
) (domain.Room, error) {
	ctx := context.Background()

	commandTag, err := s.db.Exec(
		ctx,
		`
			UPDATE rooms
			SET admin_token = $2, updated_at = now()
			WHERE id = $1
		`,
		roomID,
		newToken(),
	)
	if err != nil {
		return domain.Room{}, err
	}

	if commandTag.RowsAffected() == 0 {
		return domain.Room{}, ErrorNotFound
	}

	return s.GetRoom(roomID)
}

func (s *PostgresStore) ensureRoomExists(
	ctx context.Context,
	roomID string,
) error {
	var id string

	err := s.db.QueryRow(
		ctx,
		`SELECT id FROM rooms WHERE id = $1`,
		roomID,
	).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrorNotFound
	}

	return err
}

func (s *PostgresStore) ensureItemExists(
	ctx context.Context,
	roomID string,
	itemID string,
) error {
	var id string

	err := s.db.QueryRow(
		ctx,
		`
			SELECT id
			FROM receipt_items
			WHERE room_id = $1 AND id = $2
		`,
		roomID,
		itemID,
	).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrorItemNotFound
	}

	return err
}

func (s *PostgresStore) ensureParticipantExists(
	ctx context.Context,
	roomID string,
	participantID string,
) error {
	var id string

	err := s.db.QueryRow(
		ctx,
		`
			SELECT id
			FROM participants
			WHERE room_id = $1 AND id = $2
		`,
		roomID,
		participantID,
	).Scan(&id)

	if errors.Is(err, pgx.ErrNoRows) {
		return ErrorParticipantNotFound
	}

	return err
}

func (s *PostgresStore) ensureParticipantNameAvailable(
	ctx context.Context,
	roomID string,
	name string,
	excludeParticipantID string,
) error {
	var exists bool

	err := s.db.QueryRow(
		ctx,
		`
			SELECT EXISTS (
				SELECT 1
				FROM participants
				WHERE room_id = $1
					AND lower(name) = lower($2)
					AND id <> $3
			)
		`,
		roomID,
		name,
		excludeParticipantID,
	).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return ErrorNameTaken
	}

	return nil
}
