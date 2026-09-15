"use client";

import { useState, type SubmitEvent } from "react";
import { useRouter } from "next/navigation";

import { createRoom } from "../lib/api";
import { saveAdminToken } from "../lib/session";

export default function HomePage() {
  const router = useRouter();

  const [title, setTitle] = useState("Ужин с друзьями");
  const [currency, setCurrency] = useState("EUR");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  async function handleCreateRoom(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    setError("");
    setLoading(true);

    try {
      const result = await createRoom({
        title: title.trim(),
        currency,
      });

      saveAdminToken(result.room.id, result.admin_token);

      router.push(`/rooms/${result.room.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка создания комнаты");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="landing">
      <section className="hero">
        <p className="eyebrow">Совместное деление счёта</p>

        <h1>SplitTheBill</h1>

        <p className="hero-text">
          Создай комнату, добавь позиции из чека и отправь друзьям одну ссылку.
          Каждый отметит свои блюда сам.
        </p>
      </section>

      <section className="card create-card">
        <h2>Создать комнату</h2>

        <form onSubmit={handleCreateRoom} className="grid">
          <label>
            Название
            <input
              value={title}
              onChange={(event) => setTitle(event.target.value)}
              placeholder="Ужин с друзьями"
              maxLength={120}
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
