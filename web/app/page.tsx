"use client";

import { useState, type SubmitEvent } from "react";
import { useRouter } from "next/navigation";
import { createRoom } from "../lib/api";

export default function HomePage() {
  const router = useRouter();

  const [title, setTitle] = useState("Dinner at Bar");
  const [currency, setCurrency] = useState("EUR");

  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleCreateRoom(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    setError("");
    setLoading(true);

    try {
      const room = await createRoom({
        title,
        currency,
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
      <h1>SplitTheBill</h1>
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
          {error && <p className="error">{error}</p>}

          <button type="submit" disabled={loading || !title.trim()}>
            {loading ? "Создаём..." : "Создать комнату"}
          </button>
        </form>
      </section>
    </main>
  );
}
