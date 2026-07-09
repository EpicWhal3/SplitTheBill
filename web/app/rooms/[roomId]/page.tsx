import { useEffect, useMemo, useState } from "react";
import {
  addAssignment,
  addItem,
  addParticipant,
  calculateRoom,
  CalculateResponse,
  getRoom,
  ItemAssignment,
  Participant,
  ReceiptItem,
  Room,
  updateRoom,
} from "../../../lib/api";
import { formatMoney, parseMoneyToMinorUnits } from "../../../lib/money";

type Props = {
  params: Promise<{
    roomId: string;
  }>;
};

export default function RoomPage({ params }: Props) {
  const [roomId, setRoomId] = useState("");

  const [room, setRoom] = useState<Room | null>(null);
  const [participants, setParticipants] = useState<Participant[]>([]);
  const [items, setItems] = useState<ReceiptItem[]>([]);
  const [assignments, setAssignments] = useState<ItemAssignment[]>([]);
  const [calculation, setCalculation] = useState<CalculateResponse | null>(
    null,
  );

  const [participantName, setParticipantName] = useState("");

  const [itemName, setItemName] = useState("");
  const [itemQuantity, setItemQuantity] = useState("1");
  const [itemPrice, setItemPrice] = useState("");

  const [selectedItemId, setSelectedItemId] = useState("");
  const [selectedParticipantId, setSelectedParticipantId] = useState("");
  const [weight, setWeight] = useState("1");

  const [serviceFee, setServiceFee] = useState("0");
  const [tipAmount, setTipAmount] = useState("0");
  const [discount, setDiscount] = useState("0");

  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    params.then((resolved) => {
      setRoomId(resolved.roomId);
    });
  }, [params]);

  useEffect(() => {
    if (roomId) {
      loadRoom(roomId);
    }
  }, [roomId]);

  async function loadRoom(id: string) {
    setError("");

    try {
      const data = await getRoom(id);

      setRoom(data.room);
      setParticipants(data.participants);
      setItems(data.items);
      setAssignments(data.assignments);

      setServiceFee(String(data.room.service_fee / 100));
      setTipAmount(String(data.room.tip_amount / 100));
      setDiscount(String(data.room.discount / 100));

      if (data.items.length > 0 && !selectedItemId) {
        setSelectedItemId(data.items[0].id);
      }

      if (data.participants.length > 0 && !selectedParticipantId) {
        setSelectedParticipantId(data.participants[0].id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка загрузки комнаты");
    }
  }

  async function handleUpdateCharges(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!roomId) return;

    setLoading(true);
    setError("");

    try {
      await updateRoom(roomId, {
        service_fee: parseMoneyToMinorUnits(serviceFee),
        tip_amount: parseMoneyToMinorUnits(tipAmount),
        discount: parseMoneyToMinorUnits(discount),
      });

      await loadRoom(roomId);
      setCalculation(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка обновления счёта");
    } finally {
      setLoading(false);
    }
  }

  async function handleAddParticipant(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!roomId || !participantName.trim()) return;

    setLoading(true);
    setError("");

    try {
      await addParticipant(roomId, {
        name: participantName,
      });

      setParticipantName("");
      await loadRoom(roomId);
      setCalculation(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Ошибка добавления участника",
      );
    } finally {
      setLoading(false);
    }
  }

  async function handleAddItem(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!roomId || !itemName.trim()) return;

    const quantity = Number(itemQuantity);
    const unitPrice = parseMoneyToMinorUnits(itemPrice);

    if (!quantity || quantity <= 0) {
      setError("Количество должно быть больше 0");
      return;
    }

    if (!unitPrice || unitPrice <= 0) {
      setError("Цена должна быть больше 0");
      return;
    }

    setLoading(true);
    setError("");

    try {
      await addItem(roomId, {
        name: itemName,
        quantity,
        unit_price: unitPrice,
      });

      setItemName("");
      setItemQuantity("1");
      setItemPrice("");

      await loadRoom(roomId);
      setCalculation(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Ошибка добавления позиции",
      );
    } finally {
      setLoading(false);
    }
  }

  async function handleAddAssignment(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!roomId || !selectedItemId || !selectedParticipantId) return;

    const numericWeight = Number(weight);

    if (!numericWeight || numericWeight <= 0) {
      setError("Вес должен быть больше 0");
      return;
    }

    setLoading(true);
    setError("");

    try {
      await addAssignment(roomId, {
        item_id: selectedItemId,
        participant_id: selectedParticipantId,
        weight: numericWeight,
      });

      await loadRoom(roomId);
      setCalculation(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Ошибка распределения позиции",
      );
    } finally {
      setLoading(false);
    }
  }

  async function handleCalculate() {
    if (!roomId) return;

    setLoading(true);
    setError("");

    try {
      const result = await calculateRoom(roomId);
      setCalculation(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка расчёта");
    } finally {
      setLoading(false);
    }
  }

  const assignmentRows = useMemo(() => {
    return assignments.map((assignment) => {
      const item = items.find((item) => item.id === assignment.item_id);
      const participant = participants.find(
        (participant) => participant.id === assignment.participant_id,
      );

      return {
        ...assignment,
        itemName: item?.name ?? assignment.item_id,
        participantName: participant?.name ?? assignment.participant_id,
      };
    });
  }, [assignments, items, participants]);

  if (!room) {
    return (
      <main>
        <h1>Комната счёта</h1>
        {error ? <p className="error">{error}</p> : <p>Загрузка...</p>}
      </main>
    );
  }

  return (
    <main>
      <h1>{room.title}</h1>
      <p className="muted">
        ID комнаты: <code>{room.id}</code>
      </p>

      {error && <p className="error">{error}</p>}

      <section className="card">
        <h2>Дополнительные суммы</h2>

        <form onSubmit={handleUpdateCharges} className="grid grid-3">
          <label>
            Сервисный сбор
            <input
              value={serviceFee}
              onChange={(event) => setServiceFee(event.target.value)}
            />
          </label>

          <label>
            Чаевые
            <input
              value={tipAmount}
              onChange={(event) => setTipAmount(event.target.value)}
            />
          </label>

          <label>
            Скидка
            <input
              value={discount}
              onChange={(event) => setDiscount(event.target.value)}
            />
          </label>

          <button disabled={loading}>Сохранить суммы</button>
        </form>
      </section>

      <section className="card">
        <h2>Участники</h2>

        <form onSubmit={handleAddParticipant} className="grid grid-2">
          <label>
            Имя участника
            <input
              value={participantName}
              onChange={(event) => setParticipantName(event.target.value)}
              placeholder="Аня"
            />
          </label>

          <button disabled={loading || !participantName.trim()}>
            Добавить участника
          </button>
        </form>

        {participants.length === 0 ? (
          <p className="muted">Пока нет участников.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Имя</th>
                <th>ID</th>
              </tr>
            </thead>
            <tbody>
              {participants.map((participant) => (
                <tr key={participant.id}>
                  <td>{participant.name}</td>
                  <td>
                    <code>{participant.id}</code>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="card">
        <h2>Позиции чека</h2>

        <form onSubmit={handleAddItem} className="grid grid-3">
          <label>
            Название
            <input
              value={itemName}
              onChange={(event) => setItemName(event.target.value)}
              placeholder="Burger"
            />
          </label>

          <label>
            Количество
            <input
              value={itemQuantity}
              onChange={(event) => setItemQuantity(event.target.value)}
              placeholder="1"
            />
          </label>

          <label>
            Цена за штуку
            <input
              value={itemPrice}
              onChange={(event) => setItemPrice(event.target.value)}
              placeholder="12.50"
            />
          </label>

          <button disabled={loading || !itemName.trim()}>
            Добавить позицию
          </button>
        </form>

        {items.length === 0 ? (
          <p className="muted">Пока нет позиций.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Название</th>
                <th>Кол-во</th>
                <th>Цена</th>
                <th>Итого</th>
              </tr>
            </thead>
            <tbody>
              {items.map((item) => (
                <tr key={item.id}>
                  <td>{item.name}</td>
                  <td>{item.quantity}</td>
                  <td>{formatMoney(item.unit_price, room.currency)}</td>
                  <td>{formatMoney(item.total, room.currency)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="card">
        <h2>Распределение позиций</h2>

        <form onSubmit={handleAddAssignment} className="grid grid-3">
          <label>
            Позиция
            <select
              value={selectedItemId}
              onChange={(event) => setSelectedItemId(event.target.value)}
            >
              <option value="">Выбери позицию</option>
              {items.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name} — {formatMoney(item.total, room.currency)}
                </option>
              ))}
            </select>
          </label>

          <label>
            Участник
            <select
              value={selectedParticipantId}
              onChange={(event) => setSelectedParticipantId(event.target.value)}
            >
              <option value="">Выбери участника</option>
              {participants.map((participant) => (
                <option key={participant.id} value={participant.id}>
                  {participant.name}
                </option>
              ))}
            </select>
          </label>

          <label>
            Вес
            <input
              value={weight}
              onChange={(event) => setWeight(event.target.value)}
              placeholder="1"
            />
          </label>

          <button
            disabled={loading || !selectedItemId || !selectedParticipantId}
          >
            Назначить
          </button>
        </form>

        <p className="muted">
          Если позицию делят двое поровну — добавь две записи с весом 1. Если
          один ел в 2 раза больше — поставь ему вес 2, второму вес 1.
        </p>

        {assignmentRows.length === 0 ? (
          <p className="muted">Пока позиции не распределены.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Позиция</th>
                <th>Участник</th>
                <th>Вес</th>
              </tr>
            </thead>
            <tbody>
              {assignmentRows.map((assignment, index) => (
                <tr
                  key={`${assignment.item_id}-${assignment.participant_id}-${index}`}
                >
                  <td>{assignment.itemName}</td>
                  <td>{assignment.participantName}</td>
                  <td>{assignment.weight}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>

      <section className="card">
        <h2>Итог</h2>

        <button onClick={handleCalculate} disabled={loading}>
          Рассчитать
        </button>

        {calculation && (
          <>
            <p className="success">
              Общий итог:{" "}
              {formatMoney(calculation.calculated_total, room.currency)}
            </p>

            <table>
              <thead>
                <tr>
                  <th>Участник</th>
                  <th>Позиции</th>
                  <th>Сервис</th>
                  <th>Чаевые</th>
                  <th>Скидка</th>
                  <th>Итого</th>
                </tr>
              </thead>
              <tbody>
                {calculation.results.map((result) => (
                  <tr key={result.participant_id}>
                    <td>{result.name}</td>
                    <td>{formatMoney(result.base_amount, room.currency)}</td>
                    <td>{formatMoney(result.service_share, room.currency)}</td>
                    <td>{formatMoney(result.tip_share, room.currency)}</td>
                    <td>
                      -{formatMoney(result.discount_share, room.currency)}
                    </td>
                    <td>
                      <strong>
                        {formatMoney(result.total_amount, room.currency)}
                      </strong>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </>
        )}
      </section>
    </main>
  );
}
