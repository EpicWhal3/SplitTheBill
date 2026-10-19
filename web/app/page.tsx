"use client";

import { type SubmitEvent, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { createRoom, restoreOrganizerRooms } from "@/lib/api";
import { translateError } from "@/lib/errors";
import {
  loadOrganizerName,
  loadRecentRooms,
  removeRecentRoom,
  saveAdminToken,
  saveOrganizerName,
  saveRecentRoom,
  type RecentRoom,
} from "@/lib/session";

export default function HomePage() {
  const router = useRouter();

  const [title, setTitle] = useState("Ужин с друзьями");
  const [currency, setCurrency] = useState("EUR");
  const [organizerName, setOrganizerName] = useState("");
  const [organizerPin, setOrganizerPin] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const [recentRooms, setRecentRooms] = useState<RecentRoom[]>([]);
  const [restoreName, setRestoreName] = useState("");
  const [restorePin, setRestorePin] = useState("");
  const [restoreError, setRestoreError] = useState("");
  const [restoreMessage, setRestoreMessage] = useState("");
  const [restoreLoading, setRestoreLoading] = useState(false);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setRecentRooms(loadRecentRooms());
    setOrganizerName(loadOrganizerName());
  }, []);

  async function handleCreateRoom(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    setError("");
    setLoading(true);

    const name = organizerName.trim();

    if (name && organizerPin.length < 4) {
      setError("Пароль организатора должен быть от 4 до 64 символов.");
      setLoading(false);
      return;
    }

    try {
      const result = await createRoom({
        title: title.trim(),
        currency,
        organizer_name: name || undefined,
        organizer_pin: name ? organizerPin : undefined,
      });

      saveAdminToken(result.room.id, result.admin_token);
      saveRecentRoom({ roomId: result.room.id, title: result.room.title });
      saveOrganizerName(name);
      setRecentRooms(loadRecentRooms());

      router.push(`/rooms/${result.room.id}`);
    } catch (err) {
      setError(
        translateError(
          err instanceof Error ? err.message : "Ошибка создания комнаты",
        ),
      );
    } finally {
      setLoading(false);
    }
  }

  async function handleRestoreRooms(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    setRestoreError("");
    setRestoreMessage("");
    setRestoreLoading(true);

    try {
      const result = await restoreOrganizerRooms({
        organizer_name: restoreName.trim(),
        organizer_pin: restorePin,
      });

      for (const entry of result.rooms) {
        saveAdminToken(entry.room.id, entry.admin_token);
        saveRecentRoom({ roomId: entry.room.id, title: entry.room.title });
      }

      saveOrganizerName(restoreName);
      setRecentRooms(loadRecentRooms());

      setRestoreMessage(
        result.rooms.length > 0
          ? `Найдено комнат: ${result.rooms.length}. Откройте нужную комнату ниже.`
          : "Комнаты с такими именем и паролем не найдены.",
      );
    } catch (err) {
      setRestoreError(
        translateError(
          err instanceof Error
            ? err.message
            : "Не удалось восстановить комнаты",
        ),
      );
    } finally {
      setRestoreLoading(false);
    }
  }

  function handleRemoveRoom(roomId: string) {
    removeRecentRoom(roomId);
    setRecentRooms(loadRecentRooms());
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

          <label>
            Ваше имя (логин организатора)
            <input
              value={organizerName}
              onChange={(event) => setOrganizerName(event.target.value)}
              placeholder="Например: Аня"
              maxLength={80}
            />
          </label>

          <label>
            Пароль / PIN организатора (мин. 4 символа)
            <input
              type="password"
              value={organizerPin}
              onChange={(event) => setOrganizerPin(event.target.value)}
              placeholder="Минимум 4 символа"
              maxLength={64}
              autoComplete="new-password"
            />
          </label>

          {error && <p className="error">{error}</p>}

          <button type="submit" disabled={loading || !title.trim()}>
            {loading ? "Создаём..." : "Создать комнату"}
          </button>
        </form>
      </section>

      <section className="card recent-card">
        <h2>Мои комнаты</h2>

        {recentRooms.length === 0 ? (
          <p className="muted">
            Здесь появятся комнаты, которые вы создавали или восстановили на
            этом устройстве.
          </p>
        ) : (
          <ul className="recent-rooms">
            {recentRooms.map((room) => (
              <li key={room.roomId}>
                <Link href={`/rooms/${room.roomId}`}>
                  <span className="recent-room-title">{room.title}</span>
                  <span className="recent-room-id">{room.roomId}</span>
                </Link>
                <button
                  type="button"
                  className="secondary"
                  onClick={() => handleRemoveRoom(room.roomId)}
                >
                  Убрать
                </button>
              </li>
            ))}
          </ul>
        )}

        <details className="details-block">
          <summary>Очистили кэш или зашли с другого устройства?</summary>

          <p className="muted">
            Введите имя и пароль организатора, чтобы восстановить все ваши
            комнаты с правами организатора.
          </p>

          <form onSubmit={handleRestoreRooms} className="grid">
            <label>
              Имя организатора
              <input
                value={restoreName}
                onChange={(event) => setRestoreName(event.target.value)}
                placeholder="Например: Аня"
                maxLength={80}
              />
            </label>

            <label>
              Пароль / PIN организатора
              <input
                type="password"
                value={restorePin}
                onChange={(event) => setRestorePin(event.target.value)}
                placeholder="Минимум 4 символа"
                maxLength={64}
              />
            </label>

            {restoreError && <p className="error">{restoreError}</p>}
            {restoreMessage && <p className="success">{restoreMessage}</p>}

            <button
              type="submit"
              disabled={restoreLoading || !restoreName.trim() || !restorePin}
            >
              {restoreLoading ? "Ищем..." : "Найти мои комнаты"}
            </button>
          </form>
        </details>
      </section>
    </main>
  );
}
