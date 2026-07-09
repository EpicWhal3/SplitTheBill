import { useState } from "react";
import { useRouter } from "next/navigation";
import { createRoom } from "../lib/api";
import { parseMoneyToMinorUnits } from "../lib/money";

export default function HomePage() {
  const router = useRouter();

  const [title, setTitle] = useState("Dinner at Bar");
  const [currency, setCurrency] = useState("EUR");
  const [serviceFee, setServiceFee] = useState("0");
  const [tipAmount, setTipAmount] = useState("0");
  const [discount, setDiscount] = useState("0");

  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleCreateRoom(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    setError("");
    setLoading(true);

    try {
      const room = await createRoom({
        title,
        currency,
        service_fee: parseMoneyToMinorUnits(serviceFee),
        tip_amount: parseMoneyToMinorUnits(tipAmount),
        discount: parseMoneyToMinorUnits(discount),
      });

      router.push(`/rooms/${room.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка создания комнаты");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main>
      <h1>SplitCheck</h1>
      <p className="muted">
        Создай комнату счёта, добавь позиции и раздели их между участниками.
      </p>

      <section className="card">
        <h2>Создать счёт</h2>

        <form onSubmit={handleCreateRoom} className="grid">
          <label>
            Название
            <input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder="Dinner at Bar"
            />
          </label>

          <label>
            Валюта
            <select
              value={currency}
              onChange={(event) => setCurrency(event.target.value)}
            >
              <option value="EUR">EUR</option>
              <option value="RUB">RUB</option>
              <option value="USD">USD</option>
              <option value="GBP">GBP</option>
            </select>
          </label>

          <div className="grid grid-3">
            <label>
              Сервисный сбор
              <input
                value={serviceFee}
                onChange={(event) => setServiceFee(event.target.value)}
                placeholder="0"
              />
            </label>

            <label>
              Чаевые
              <input
                value={tipAmount}
                onChange={(event) => setTipAmount(event.target.value)}
                placeholder="0"
              />
            </label>

            <label>
              Скидка
              <input
                value={discount}
                onChange={(event) => setDiscount(event.target.value)}
                placeholder="0"
              />
            </label>
          </div>

          {error && <p className="error">{error}</p>}

          <button disabled={loading || !title.trim()}>
            {loading ? "Создаём..." : "Создать комнату"}
          </button>
        </form>
      </section>
    </main>
  );
}
