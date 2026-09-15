"use client";

import Image from "next/image";
import * as QRCode from "qrcode";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type SubmitEvent,
} from "react";

import {
  addAssignment,
  addItem,
  addParticipant,
  calculateRoom,
  type CalculateResponse,
  deleteAssignment,
  deleteItem,
  deleteParticipant,
  finalizeRoom,
  getRoom,
  type ItemAssignment,
  joinRoom,
  openRoomSelections,
  type Participant,
  type ReceiptItem,
  reopenRoom,
  type Room,
  selectItem,
  unselectItem,
  updateItem,
  updateParticipant,
  updateRoom,
} from "../../../lib/api";
import { calculateParticipantPreview } from "../../../lib/local-calculation";
import { formatMoney, tryParseMoneyToMinorUnits } from "../../../lib/money";
import {
  clearAdminToken,
  clearParticipantSession,
  loadAdminToken,
  loadParticipantSession,
  saveParticipantSession,
  type ParticipantSession,
} from "../../../lib/session";

type Props = {
  params: Promise<{
    roomId: string;
  }>;
};

type WeightDrafts = Record<string, string>;

export default function RoomPage({ params }: Props) {
  const [roomId, setRoomId] = useState("");
  const [room, setRoom] = useState<Room | null>(null);
  const [participants, setParticipants] = useState<Participant[]>([]);
  const [items, setItems] = useState<ReceiptItem[]>([]);
  const [assignments, setAssignments] = useState<ItemAssignment[]>([]);
  const [unassignedItemIds, setUnassignedItemIds] = useState<string[]>([]);
  const [calculation, setCalculation] = useState<CalculateResponse | null>(
    null,
  );

  const [adminToken, setAdminToken] = useState("");
  const [participantSession, setParticipantSession] =
    useState<ParticipantSession | null>(null);
  const [authReady, setAuthReady] = useState(false);

  const [joinName, setJoinName] = useState("");
  const [participantName, setParticipantName] = useState("");
  const [itemName, setItemName] = useState("");
  const [itemQuantity, setItemQuantity] = useState("1");
  const [itemPrice, setItemPrice] = useState("");
  const [selectedItemId, setSelectedItemId] = useState("");
  const [selectedParticipantId, setSelectedParticipantId] = useState("");
  const [adminWeight, setAdminWeight] = useState("1");
  const [participantWeights, setParticipantWeights] = useState<WeightDrafts>(
    {},
  );

  const [serviceFee, setServiceFee] = useState("0");
  const [tipAmount, setTipAmount] = useState("0");
  const [discount, setDiscount] = useState("0");
  const [discountMode, setDiscountMode] = useState<"proportional" | "equal">(
    "proportional",
  );
  const [expectedTotal, setExpectedTotal] = useState("0");
  const [payerParticipantId, setPayerParticipantId] = useState("");

  const [shareUrl, setShareUrl] = useState("");
  const [qrCodeUrl, setQrCodeUrl] = useState("");
  const [copyState, setCopyState] = useState("");
  const [lastUpdatedAt, setLastUpdatedAt] = useState<Date | null>(null);

  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [selectionLoadingId, setSelectionLoadingId] = useState("");

  const loadRoomData = useCallback(
    async (id: string, syncForm: boolean, silent = false) => {
      if (!silent) {
        setError("");
      }

      try {
        const data = await getRoom(id);
        const nextParticipants = data.participants ?? [];
        const nextItems = data.items ?? [];
        const nextAssignments = data.assignments ?? [];

        setRoom(data.room);
        setParticipants(nextParticipants);
        setItems(nextItems);
        setAssignments(nextAssignments);
        setUnassignedItemIds(data.unassigned_item_ids ?? []);
        setLastUpdatedAt(new Date());

        setSelectedItemId((current) =>
          nextItems.some((item) => item.id === current)
            ? current
            : (nextItems[0]?.id ?? ""),
        );

        setSelectedParticipantId((current) =>
          nextParticipants.some((participant) => participant.id === current)
            ? current
            : (nextParticipants[0]?.id ?? ""),
        );

        if (syncForm) {
          setServiceFee(String(data.room.service_fee / 100));
          setTipAmount(String(data.room.tip_amount / 100));
          setDiscount(String(data.room.discount / 100));
          setDiscountMode(data.room.discount_mode);
          setExpectedTotal(String(data.room.expected_total / 100));
          setPayerParticipantId(data.room.payer_participant_id ?? "");
        }
      } catch (err) {
        if (!silent) {
          setError(
            err instanceof Error ? err.message : "Ошибка загрузки комнаты",
          );
        }
      }
    },
    [],
  );

  useEffect(() => {
    params.then((resolved) => {
      const id = resolved.roomId;

      setRoomId(id);
      setAdminToken(loadAdminToken(id));
      setParticipantSession(loadParticipantSession(id));
      setShareUrl(`${window.location.origin}/rooms/${id}`);
      setAuthReady(true);
    });
  }, [params]);

  useEffect(() => {
    if (roomId) {
      void loadRoomData(roomId, true);
    }
  }, [loadRoomData, roomId]);

  useEffect(() => {
    if (!roomId) {
      return;
    }

    const intervalId = window.setInterval(() => {
      void loadRoomData(roomId, false, true);
    }, 3000);

    return () => window.clearInterval(intervalId);
  }, [loadRoomData, roomId]);

  useEffect(() => {
    if (!shareUrl) {
      return;
    }

    let cancelled = false;

    QRCode.toDataURL(shareUrl, {
      width: 220,
      margin: 1,
      errorCorrectionLevel: "M",
    })
      .then((url: string) => {
        if (!cancelled) {
          setQrCodeUrl(url);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setQrCodeUrl("");
        }
      });

    return () => {
      cancelled = true;
    };
  }, [shareUrl]);

  useEffect(() => {
    if (
      !room ||
      !participantSession ||
      participants.some(
        (participant) => participant.id === participantSession.participantId,
      )
    ) {
      return;
    }

    clearParticipantSession(room.id);
    setParticipantSession(null);
    setError(
      "Участник был удалён из комнаты. Войдите снова под другим именем.",
    );
  }, [participantSession, participants, room]);

  useEffect(() => {
    if (!participantSession) {
      setParticipantWeights({});
      return;
    }

    const next: WeightDrafts = {};
    for (const assignment of assignments) {
      if (assignment.participant_id === participantSession.participantId) {
        next[assignment.item_id] = String(assignment.weight);
      }
    }
    setParticipantWeights(next);
  }, [assignments, participantSession]);

  useEffect(() => {
    if (!roomId || room?.status !== "finalized") {
      return;
    }

    void calculateRoom(roomId)
      .then(setCalculation)
      .catch((err: unknown) => {
        setError(
          err instanceof Error ? err.message : "Не удалось загрузить итог",
        );
      });
  }, [room?.finalized_at, room?.status, roomId]);

  async function runAdminMutation(
    action: () => Promise<unknown>,
  ): Promise<boolean> {
    if (!roomId || !adminToken) {
      setError("В этом браузере нет ключа организатора.");
      return false;
    }

    setLoading(true);
    setError("");

    try {
      await action();
      await loadRoomData(roomId, true);
      setCalculation(null);
      return true;
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Ошибка выполнения операции";

      setError(message);
      if (message === "organizer access required") {
        clearAdminToken(roomId);
        setAdminToken("");
      }
      return false;
    } finally {
      setLoading(false);
    }
  }

  function parseRequiredMoney(value: string, fieldName: string): number | null {
    const parsed = tryParseMoneyToMinorUnits(value);

    if (parsed === null) {
      setError(`Поле «${fieldName}» должно содержать число`);
      return null;
    }

    return parsed;
  }

  async function handleJoin(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const name = joinName.trim();
    if (!roomId || !name) {
      return;
    }

    setLoading(true);
    setError("");

    try {
      const result = await joinRoom(roomId, { name });
      const session: ParticipantSession = {
        participantId: result.participant.id,
        participantToken: result.participant_token,
        name: result.participant.name,
      };

      saveParticipantSession(roomId, session);
      setParticipantSession(session);
      setJoinName("");
      await loadRoomData(roomId, false);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Не удалось войти в комнату",
      );
    } finally {
      setLoading(false);
    }
  }

  async function saveParticipantWeight(item: ReceiptItem, weightValue: string) {
    if (!participantSession || !roomId) {
      return;
    }

    const numericWeight = Number(weightValue);
    if (
      !Number.isInteger(numericWeight) ||
      numericWeight < 1 ||
      numericWeight > 1000
    ) {
      setError("Вес должен быть целым числом от 1 до 1000");
      return;
    }

    setSelectionLoadingId(item.id);
    setError("");

    try {
      await selectItem(
        roomId,
        item.id,
        participantSession.participantToken,
        numericWeight,
      );
      await loadRoomData(roomId, false);
      setCalculation(null);
    } catch (err) {
      handleParticipantError(err);
    } finally {
      setSelectionLoadingId("");
    }
  }

  async function handleToggleSelection(item: ReceiptItem) {
    if (!participantSession || !roomId) {
      return;
    }

    const selected = assignments.some(
      (assignment) =>
        assignment.item_id === item.id &&
        assignment.participant_id === participantSession.participantId,
    );

    setSelectionLoadingId(item.id);
    setError("");

    try {
      if (selected) {
        await unselectItem(
          roomId,
          item.id,
          participantSession.participantToken,
        );
      } else {
        await selectItem(
          roomId,
          item.id,
          participantSession.participantToken,
          1,
        );
      }

      await loadRoomData(roomId, false);
      setCalculation(null);
    } catch (err) {
      handleParticipantError(err);
    } finally {
      setSelectionLoadingId("");
    }
  }

  function handleParticipantError(err: unknown) {
    const message =
      err instanceof Error ? err.message : "Не удалось изменить выбор";

    setError(message);

    if (message === "participant session is invalid" && roomId) {
      clearParticipantSession(roomId);
      setParticipantSession(null);
    }
  }

  function handleLeaveParticipantMode() {
    if (!roomId) {
      return;
    }

    clearParticipantSession(roomId);
    setParticipantSession(null);
    setError("");
  }

  async function handleCopyLink() {
    if (!shareUrl) {
      return;
    }

    try {
      await navigator.clipboard.writeText(shareUrl);
      setCopyState("Ссылка скопирована");
    } catch {
      setCopyState("Не удалось скопировать автоматически");
    }

    window.setTimeout(() => setCopyState(""), 2500);
  }

  async function handleShareLink() {
    if (!shareUrl || !room) {
      return;
    }

    if (navigator.share) {
      try {
        await navigator.share({
          title: room.title,
          text: "Отметь свои позиции в общем чеке",
          url: shareUrl,
        });
        return;
      } catch {
        return;
      }
    }

    await handleCopyLink();
  }

  async function handleUpdateCharges(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const parsedServiceFee = parseRequiredMoney(serviceFee, "Сервисный сбор");
    const parsedTipAmount = parseRequiredMoney(tipAmount, "Чаевые");
    const parsedDiscount = parseRequiredMoney(discount, "Скидка");
    const parsedExpectedTotal = parseRequiredMoney(
      expectedTotal,
      "Итог по чеку",
    );

    if (
      parsedServiceFee === null ||
      parsedTipAmount === null ||
      parsedDiscount === null ||
      parsedExpectedTotal === null
    ) {
      return;
    }

    if (
      parsedServiceFee < 0 ||
      parsedTipAmount < 0 ||
      parsedDiscount < 0 ||
      parsedExpectedTotal < 0
    ) {
      setError("Дополнительные суммы не могут быть отрицательными");
      return;
    }

    await runAdminMutation(() =>
      updateRoom(roomId, adminToken, {
        service_fee: parsedServiceFee,
        tip_amount: parsedTipAmount,
        discount: parsedDiscount,
        discount_mode: discountMode,
        expected_total: parsedExpectedTotal,
        payer_participant_id: payerParticipantId,
      }),
    );
  }

  async function handleAddParticipant(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    const name = participantName.trim();
    if (!name) {
      return;
    }

    const success = await runAdminMutation(() =>
      addParticipant(roomId, adminToken, { name }),
    );
    if (success) {
      setParticipantName("");
    }
  }

  async function handleEditParticipant(participant: Participant) {
    const name = window.prompt("Новое имя участника", participant.name);
    if (name === null || !name.trim()) {
      return;
    }

    await runAdminMutation(() =>
      updateParticipant(roomId, participant.id, adminToken, {
        name: name.trim(),
      }),
    );
  }

  async function handleDeleteParticipant(participant: Participant) {
    if (
      !window.confirm(
        `Удалить участника «${participant.name}»? Его отметки также будут удалены.`,
      )
    ) {
      return;
    }

    await runAdminMutation(() =>
      deleteParticipant(roomId, participant.id, adminToken),
    );
  }

  async function handleAddItem(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const quantity = Number(itemQuantity);
    const unitPrice = parseRequiredMoney(itemPrice, "Цена за штуку");

    if (!Number.isInteger(quantity) || quantity <= 0) {
      setError("Количество должно быть положительным целым числом");
      return;
    }
    if (unitPrice === null || unitPrice <= 0 || !itemName.trim()) {
      setError("Название и положительная цена обязательны");
      return;
    }

    const success = await runAdminMutation(() =>
      addItem(roomId, adminToken, {
        name: itemName.trim(),
        quantity,
        unit_price: unitPrice,
      }),
    );

    if (success) {
      setItemName("");
      setItemQuantity("1");
      setItemPrice("");
    }
  }

  async function handleEditItem(item: ReceiptItem) {
    const name = window.prompt("Название позиции", item.name);
    if (name === null) {
      return;
    }
    const quantityText = window.prompt("Количество", String(item.quantity));
    if (quantityText === null) {
      return;
    }
    const priceText = window.prompt(
      "Цена за штуку",
      String(item.unit_price / 100),
    );
    if (priceText === null) {
      return;
    }

    const quantity = Number(quantityText);
    const unitPrice = tryParseMoneyToMinorUnits(priceText);
    if (
      !name.trim() ||
      !Number.isInteger(quantity) ||
      quantity <= 0 ||
      unitPrice === null ||
      unitPrice <= 0
    ) {
      setError("Проверьте название, количество и цену");
      return;
    }

    await runAdminMutation(() =>
      updateItem(roomId, item.id, adminToken, {
        name: name.trim(),
        quantity,
        unit_price: unitPrice,
      }),
    );
  }

  async function handleDeleteItem(item: ReceiptItem) {
    if (
      !window.confirm(
        `Удалить позицию «${item.name}»? Все отметки также будут удалены.`,
      )
    ) {
      return;
    }

    await runAdminMutation(() => deleteItem(roomId, item.id, adminToken));
  }

  async function handleAddAssignment(event: SubmitEvent<HTMLFormElement>) {
    event.preventDefault();

    const numericWeight = Number(adminWeight);
    if (
      !Number.isInteger(numericWeight) ||
      numericWeight < 1 ||
      numericWeight > 1000 ||
      !selectedItemId ||
      !selectedParticipantId
    ) {
      setError("Выберите позицию, участника и вес от 1 до 1000");
      return;
    }

    await runAdminMutation(() =>
      addAssignment(roomId, adminToken, {
        item_id: selectedItemId,
        participant_id: selectedParticipantId,
        weight: numericWeight,
      }),
    );
  }

  async function handleDeleteAssignment(assignment: ItemAssignment) {
    await runAdminMutation(() =>
      deleteAssignment(
        roomId,
        assignment.item_id,
        assignment.participant_id,
        adminToken,
      ),
    );
  }

  async function handleCalculate() {
    if (!roomId) {
      return;
    }

    setLoading(true);
    setError("");
    try {
      setCalculation(await calculateRoom(roomId));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ошибка расчёта");
    } finally {
      setLoading(false);
    }
  }

  async function handleOpenSelections() {
    await runAdminMutation(() => openRoomSelections(roomId, adminToken));
  }

  async function handleFinalize() {
    setLoading(true);
    setError("");
    try {
      const result = await finalizeRoom(roomId, adminToken);
      setCalculation(result);
      await loadRoomData(roomId, true);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Не удалось завершить распределение",
      );
    } finally {
      setLoading(false);
    }
  }

  async function handleReopen() {
    await runAdminMutation(() => reopenRoom(roomId, adminToken));
  }

  async function handleCopySummary() {
    if (!room || !calculation) {
      return;
    }

    const lines = [
      `${room.title}`,
      `Итог: ${formatMoney(calculation.calculated_total, room.currency)}`,
      "",
      ...calculation.debts.map(
        (debt) =>
          `${debt.from_name} должен(на) ${debt.to_name}: ${formatMoney(
            debt.amount,
            room.currency,
          )}`,
      ),
    ];

    await navigator.clipboard.writeText(lines.join("\n"));
    setCopyState("Итог скопирован");
    window.setTimeout(() => setCopyState(""), 2500);
  }

  const subtotal = useMemo(
    () => items.reduce((sum, item) => sum + item.total, 0),
    [items],
  );

  const payer = useMemo(
    () =>
      participants.find(
        (participant) => participant.id === room?.payer_participant_id,
      ),
    [participants, room?.payer_participant_id],
  );

  const participantPreview = useMemo(() => {
    if (!room || !participantSession) {
      return null;
    }

    return calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      participantSession.participantId,
    );
  }, [assignments, items, participantSession, participants, room]);

  const assignmentRows = useMemo(
    () =>
      assignments.map((assignment) => ({
        ...assignment,
        itemName:
          items.find((item) => item.id === assignment.item_id)?.name ??
          assignment.item_id,
        participantName:
          participants.find(
            (participant) => participant.id === assignment.participant_id,
          )?.name ?? assignment.participant_id,
      })),
    [assignments, items, participants],
  );

  if (!authReady || !room) {
    return (
      <main>
        <h1>Комната счёта</h1>
        {error ? <p className="error">{error}</p> : <p>Загрузка...</p>}
      </main>
    );
  }

  const isAdmin = Boolean(adminToken);

  return (
    <main>
      <RoomHeader
        room={room}
        role={isAdmin ? "Организатор" : "Участник"}
        lastUpdatedAt={lastUpdatedAt}
      />

      <StatusPanel
        room={room}
        isAdmin={isAdmin}
        loading={loading}
        unassignedCount={unassignedItemIds.length}
        onOpen={() => void handleOpenSelections()}
        onFinalize={() => void handleFinalize()}
        onReopen={() => void handleReopen()}
      />

      {error && <p className="error card">{translateError(error)}</p>}
      {copyState && <p className="success card">{copyState}</p>}

      {isAdmin ? (
        <AdminView
          room={room}
          participants={participants}
          items={items}
          assignmentRows={assignmentRows}
          subtotal={subtotal}
          qrCodeUrl={qrCodeUrl}
          shareUrl={shareUrl}
          loading={loading}
          serviceFee={serviceFee}
          tipAmount={tipAmount}
          discount={discount}
          discountMode={discountMode}
          expectedTotal={expectedTotal}
          payerParticipantId={payerParticipantId}
          participantName={participantName}
          itemName={itemName}
          itemQuantity={itemQuantity}
          itemPrice={itemPrice}
          selectedItemId={selectedItemId}
          selectedParticipantId={selectedParticipantId}
          adminWeight={adminWeight}
          calculation={calculation}
          unassignedItemIds={unassignedItemIds}
          onCopyLink={() => void handleCopyLink()}
          onShareLink={() => void handleShareLink()}
          onCopySummary={() => void handleCopySummary()}
          onSetServiceFee={setServiceFee}
          onSetTipAmount={setTipAmount}
          onSetDiscount={setDiscount}
          onSetDiscountMode={setDiscountMode}
          onSetExpectedTotal={setExpectedTotal}
          onSetPayerParticipantId={setPayerParticipantId}
          onSetParticipantName={setParticipantName}
          onSetItemName={setItemName}
          onSetItemQuantity={setItemQuantity}
          onSetItemPrice={setItemPrice}
          onSetSelectedItemId={setSelectedItemId}
          onSetSelectedParticipantId={setSelectedParticipantId}
          onSetAdminWeight={setAdminWeight}
          onUpdateCharges={handleUpdateCharges}
          onAddParticipant={handleAddParticipant}
          onEditParticipant={(participant) =>
            void handleEditParticipant(participant)
          }
          onDeleteParticipant={(participant) =>
            void handleDeleteParticipant(participant)
          }
          onAddItem={handleAddItem}
          onEditItem={(item) => void handleEditItem(item)}
          onDeleteItem={(item) => void handleDeleteItem(item)}
          onAddAssignment={handleAddAssignment}
          onDeleteAssignment={(assignment) =>
            void handleDeleteAssignment(assignment)
          }
          onCalculate={() => void handleCalculate()}
        />
      ) : (
        <ParticipantView
          room={room}
          participants={participants}
          items={items}
          assignments={assignments}
          participantSession={participantSession}
          participantPreview={participantPreview}
          participantWeights={participantWeights}
          calculation={calculation}
          joinName={joinName}
          loading={loading}
          selectionLoadingId={selectionLoadingId}
          payerName={payer?.name ?? ""}
          onSetJoinName={setJoinName}
          onJoin={handleJoin}
          onToggleSelection={(item) => void handleToggleSelection(item)}
          onWeightChange={(itemId, value) =>
            setParticipantWeights((current) => ({
              ...current,
              [itemId]: value,
            }))
          }
          onSaveWeight={(item, value) =>
            void saveParticipantWeight(item, value)
          }
          onLeave={handleLeaveParticipantMode}
          onRefresh={() => void loadRoomData(roomId, false)}
        />
      )}
    </main>
  );
}

type AdminViewProps = {
  room: Room;
  participants: Participant[];
  items: ReceiptItem[];
  assignmentRows: Array<
    ItemAssignment & {
      itemName: string;
      participantName: string;
    }
  >;
  subtotal: number;
  qrCodeUrl: string;
  shareUrl: string;
  loading: boolean;
  serviceFee: string;
  tipAmount: string;
  discount: string;
  discountMode: "proportional" | "equal";
  expectedTotal: string;
  payerParticipantId: string;
  participantName: string;
  itemName: string;
  itemQuantity: string;
  itemPrice: string;
  selectedItemId: string;
  selectedParticipantId: string;
  adminWeight: string;
  calculation: CalculateResponse | null;
  unassignedItemIds: string[];
  onCopyLink: () => void;
  onShareLink: () => void;
  onCopySummary: () => void;
  onSetServiceFee: (value: string) => void;
  onSetTipAmount: (value: string) => void;
  onSetDiscount: (value: string) => void;
  onSetDiscountMode: (value: "proportional" | "equal") => void;
  onSetExpectedTotal: (value: string) => void;
  onSetPayerParticipantId: (value: string) => void;
  onSetParticipantName: (value: string) => void;
  onSetItemName: (value: string) => void;
  onSetItemQuantity: (value: string) => void;
  onSetItemPrice: (value: string) => void;
  onSetSelectedItemId: (value: string) => void;
  onSetSelectedParticipantId: (value: string) => void;
  onSetAdminWeight: (value: string) => void;
  onUpdateCharges: (event: SubmitEvent<HTMLFormElement>) => void;
  onAddParticipant: (event: SubmitEvent<HTMLFormElement>) => void;
  onEditParticipant: (participant: Participant) => void;
  onDeleteParticipant: (participant: Participant) => void;
  onAddItem: (event: SubmitEvent<HTMLFormElement>) => void;
  onEditItem: (item: ReceiptItem) => void;
  onDeleteItem: (item: ReceiptItem) => void;
  onAddAssignment: (event: SubmitEvent<HTMLFormElement>) => void;
  onDeleteAssignment: (assignment: ItemAssignment) => void;
  onCalculate: () => void;
};

function AdminView(props: AdminViewProps) {
  const locked = props.room.status === "finalized";

  return (
    <>
      <section className="card share-card">
        <div>
          <p className="eyebrow">Ссылка для участников</p>
          <p className="share-url">{props.shareUrl}</p>
          <div className="actions">
            <button type="button" onClick={props.onCopyLink}>
              Копировать ссылку
            </button>
            <button
              type="button"
              className="secondary"
              onClick={props.onShareLink}
            >
              Поделиться
            </button>
          </div>
        </div>
        {props.qrCodeUrl && (
          <Image
            src={props.qrCodeUrl}
            width={180}
            height={180}
            alt="QR-код комнаты"
          />
        )}
      </section>

      <section className="card">
        <h2>Суммы и правила</h2>
        <form onSubmit={props.onUpdateCharges} className="grid grid-3">
          <MoneyInput
            label="Итог на чеке"
            value={props.expectedTotal}
            onChange={props.onSetExpectedTotal}
            disabled={locked}
          />
          <MoneyInput
            label="Сервисный сбор"
            value={props.serviceFee}
            onChange={props.onSetServiceFee}
            disabled={locked}
          />
          <MoneyInput
            label="Чаевые"
            value={props.tipAmount}
            onChange={props.onSetTipAmount}
            disabled={locked}
          />
          <MoneyInput
            label="Скидка"
            value={props.discount}
            onChange={props.onSetDiscount}
            disabled={locked}
          />

          <label>
            Как разделить скидку
            <select
              value={props.discountMode}
              disabled={locked}
              onChange={(event) =>
                props.onSetDiscountMode(
                  event.target.value as "proportional" | "equal",
                )
              }
            >
              <option value="proportional">
                Пропорционально стоимости блюд
              </option>
              <option value="equal">Поровну между участниками счёта</option>
            </select>
          </label>

          <label>
            Кто оплатил весь чек
            <select
              value={props.payerParticipantId}
              disabled={locked}
              onChange={(event) =>
                props.onSetPayerParticipantId(event.target.value)
              }
            >
              <option value="">Не выбран</option>
              {props.participants.map((participant) => (
                <option key={participant.id} value={participant.id}>
                  {participant.name}
                </option>
              ))}
            </select>
          </label>

          <button disabled={props.loading || locked}>Сохранить правила</button>
        </form>

        <p className="muted">
          При пропорциональном режиме участник получает ту же долю скидки, что и
          его доля блюд. При равном режиме скидка делится поровну, но сумма
          участника никогда не становится отрицательной.
        </p>
        <p className="muted">
          Сумма позиций: {formatMoney(props.subtotal, props.room.currency)}
        </p>
      </section>

      <section className="card">
        <h2>Участники</h2>
        <form onSubmit={props.onAddParticipant} className="grid grid-2">
          <label>
            Имя участника
            <input
              value={props.participantName}
              disabled={locked}
              maxLength={80}
              onChange={(event) =>
                props.onSetParticipantName(event.target.value)
              }
            />
          </label>
          <button disabled={props.loading || locked}>Добавить участника</button>
        </form>

        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Имя</th>
                <th>Вошёл по ссылке</th>
                <th>Действия</th>
              </tr>
            </thead>
            <tbody>
              {props.participants.map((participant) => (
                <tr key={participant.id}>
                  <td>{participant.name}</td>
                  <td>{participant.claimed ? "Да" : "Нет"}</td>
                  <td>
                    <div className="actions">
                      <button
                        type="button"
                        className="secondary"
                        disabled={locked}
                        onClick={() => props.onEditParticipant(participant)}
                      >
                        Изменить
                      </button>
                      <button
                        type="button"
                        className="danger"
                        disabled={locked}
                        onClick={() => props.onDeleteParticipant(participant)}
                      >
                        Удалить
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <h2>Позиции чека</h2>
        <form onSubmit={props.onAddItem} className="grid grid-3">
          <label>
            Название
            <input
              value={props.itemName}
              disabled={locked}
              onChange={(event) => props.onSetItemName(event.target.value)}
            />
          </label>
          <label>
            Количество
            <input
              type="number"
              min="1"
              step="1"
              value={props.itemQuantity}
              disabled={locked}
              onChange={(event) => props.onSetItemQuantity(event.target.value)}
            />
          </label>
          <MoneyInput
            label="Цена за штуку"
            value={props.itemPrice}
            onChange={props.onSetItemPrice}
            disabled={locked}
            positive
          />
          <button disabled={props.loading || locked}>Добавить позицию</button>
        </form>

        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Название</th>
                <th>Кол-во</th>
                <th>Цена</th>
                <th>Итого</th>
                <th>Статус</th>
                <th>Действия</th>
              </tr>
            </thead>
            <tbody>
              {props.items.map((item) => (
                <tr key={item.id}>
                  <td>{item.name}</td>
                  <td>{item.quantity}</td>
                  <td>{formatMoney(item.unit_price, props.room.currency)}</td>
                  <td>{formatMoney(item.total, props.room.currency)}</td>
                  <td>
                    {props.unassignedItemIds.includes(item.id) ? (
                      <span className="status-warning">Не распределено</span>
                    ) : (
                      <span className="success">Распределено</span>
                    )}
                  </td>
                  <td>
                    <div className="actions">
                      <button
                        type="button"
                        className="secondary"
                        disabled={locked}
                        onClick={() => props.onEditItem(item)}
                      >
                        Изменить
                      </button>
                      <button
                        type="button"
                        className="danger"
                        disabled={locked}
                        onClick={() => props.onDeleteItem(item)}
                      >
                        Удалить
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <h2>Распределение позиций</h2>
        <form onSubmit={props.onAddAssignment} className="grid grid-3">
          <label>
            Позиция
            <select
              value={props.selectedItemId}
              disabled={locked}
              onChange={(event) =>
                props.onSetSelectedItemId(event.target.value)
              }
            >
              <option value="">Выберите позицию</option>
              {props.items.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Участник
            <select
              value={props.selectedParticipantId}
              disabled={locked}
              onChange={(event) =>
                props.onSetSelectedParticipantId(event.target.value)
              }
            >
              <option value="">Выберите участника</option>
              {props.participants.map((participant) => (
                <option key={participant.id} value={participant.id}>
                  {participant.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Вес
            <input
              type="number"
              min="1"
              max="1000"
              step="1"
              value={props.adminWeight}
              disabled={locked}
              onChange={(event) => props.onSetAdminWeight(event.target.value)}
            />
          </label>
          <button disabled={props.loading || locked}>
            Сохранить назначение
          </button>
        </form>

        <p className="muted">
          Вес 2 означает вдвое большую долю общего блюда относительно веса 1.
        </p>

        <div className="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Позиция</th>
                <th>Участник</th>
                <th>Вес</th>
                <th>Действие</th>
              </tr>
            </thead>
            <tbody>
              {props.assignmentRows.map((assignment) => (
                <tr key={`${assignment.item_id}:${assignment.participant_id}`}>
                  <td>{assignment.itemName}</td>
                  <td>{assignment.participantName}</td>
                  <td>{assignment.weight}</td>
                  <td>
                    <button
                      type="button"
                      className="danger"
                      disabled={locked}
                      onClick={() => props.onDeleteAssignment(assignment)}
                    >
                      Снять
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="card">
        <div className="section-title-row">
          <div>
            <p className="eyebrow">Проверка</p>
            <h2>Расчёт и долги</h2>
          </div>
          <div className="actions">
            <button
              type="button"
              className="secondary"
              disabled={props.loading}
              onClick={props.onCalculate}
            >
              Предварительный расчёт
            </button>
            {props.calculation && (
              <button type="button" onClick={props.onCopySummary}>
                Копировать итог
              </button>
            )}
          </div>
        </div>

        {props.calculation ? (
          <CalculationResult
            calculation={props.calculation}
            room={props.room}
          />
        ) : (
          <p className="muted">
            Расчёт доступен, когда каждая позиция назначена хотя бы одному
            участнику.
          </p>
        )}
      </section>
    </>
  );
}

type ParticipantViewProps = {
  room: Room;
  participants: Participant[];
  items: ReceiptItem[];
  assignments: ItemAssignment[];
  participantSession: ParticipantSession | null;
  participantPreview: ReturnType<typeof calculateParticipantPreview> | null;
  participantWeights: WeightDrafts;
  calculation: CalculateResponse | null;
  joinName: string;
  loading: boolean;
  selectionLoadingId: string;
  payerName: string;
  onSetJoinName: (value: string) => void;
  onJoin: (event: SubmitEvent<HTMLFormElement>) => void;
  onToggleSelection: (item: ReceiptItem) => void;
  onWeightChange: (itemId: string, value: string) => void;
  onSaveWeight: (item: ReceiptItem, value: string) => void;
  onLeave: () => void;
  onRefresh: () => void;
};

function ParticipantView(props: ParticipantViewProps) {
  if (!props.participantSession) {
    return (
      <section className="card join-card">
        <p className="eyebrow">Вход в комнату</p>
        <h2>Как вас зовут?</h2>
        {props.room.status === "finalized" ? (
          <p>Распределение уже завершено. Войти новым участником нельзя.</p>
        ) : (
          <form onSubmit={props.onJoin} className="grid grid-2">
            <label>
              Имя
              <input
                value={props.joinName}
                onChange={(event) => props.onSetJoinName(event.target.value)}
              />
            </label>
            <button disabled={props.loading || !props.joinName.trim()}>
              Войти
            </button>
          </form>
        )}
      </section>
    );
  }

  if (props.room.status === "finalized") {
    return (
      <>
        <section className="card">
          <p className="eyebrow">Распределение завершено</p>
          <h2>Финальный результат</h2>
          {props.calculation ? (
            <CalculationResult
              calculation={props.calculation}
              room={props.room}
              focusParticipantId={props.participantSession.participantId}
            />
          ) : (
            <p>Загрузка результата...</p>
          )}
        </section>
        <ParticipantFooter
          onRefresh={props.onRefresh}
          onLeave={props.onLeave}
        />
      </>
    );
  }

  if (props.room.status === "draft") {
    return (
      <>
        <section className="card waiting-card">
          <p className="eyebrow">Вы вошли как</p>
          <h2>{props.participantSession.name}</h2>
          <p>
            Организатор ещё заполняет чек. Выбор блюд станет доступен после
            открытия распределения.
          </p>
        </section>
        <ParticipantFooter
          onRefresh={props.onRefresh}
          onLeave={props.onLeave}
        />
      </>
    );
  }

  return (
    <>
      <section className="card participant-intro">
        <div>
          <p className="eyebrow">Вы вошли как</p>
          <h2>{props.participantSession.name}</h2>
        </div>
        {props.payerName && <p>Чек оплатил: {props.payerName}</p>}
      </section>

      <section className="item-grid">
        {props.items.map((item) => {
          const itemAssignments = props.assignments.filter(
            (assignment) => assignment.item_id === item.id,
          );
          const ownAssignment = itemAssignments.find(
            (assignment) =>
              assignment.participant_id ===
              props.participantSession?.participantId,
          );
          const selectedNames = itemAssignments
            .map((assignment) => {
              const participant = props.participants.find(
                (value) => value.id === assignment.participant_id,
              );
              return participant
                ? `${participant.name} (${assignment.weight})`
                : null;
            })
            .filter((name): name is string => Boolean(name));

          return (
            <article
              key={item.id}
              className={`item-card ${
                ownAssignment ? "item-card-selected" : ""
              }`}
            >
              <div className="item-card-top">
                <div>
                  <h3>{item.name}</h3>
                  {item.quantity > 1 && (
                    <p className="muted">
                      {item.quantity} ×{" "}
                      {formatMoney(item.unit_price, props.room.currency)}
                    </p>
                  )}
                </div>
                <strong>{formatMoney(item.total, props.room.currency)}</strong>
              </div>

              <div className="selected-by">
                {selectedNames.length > 0 ? (
                  <p>Выбрали: {selectedNames.join(", ")}</p>
                ) : (
                  <p className="muted">Пока никто не выбрал</p>
                )}
              </div>

              {ownAssignment && (
                <div className="weight-control">
                  <label>
                    Мой вес
                    <input
                      type="number"
                      min="1"
                      max="1000"
                      step="1"
                      value={
                        props.participantWeights[item.id] ??
                        String(ownAssignment.weight)
                      }
                      onChange={(event) =>
                        props.onWeightChange(item.id, event.target.value)
                      }
                    />
                  </label>
                  <button
                    type="button"
                    className="secondary"
                    disabled={props.selectionLoadingId === item.id}
                    onClick={() =>
                      props.onSaveWeight(
                        item,
                        props.participantWeights[item.id] ?? "1",
                      )
                    }
                  >
                    Сохранить вес
                  </button>
                </div>
              )}

              <button
                type="button"
                className={ownAssignment ? "selected-button" : ""}
                disabled={props.selectionLoadingId === item.id}
                onClick={() => props.onToggleSelection(item)}
              >
                {props.selectionLoadingId === item.id
                  ? "Сохраняем..."
                  : ownAssignment
                    ? "✓ Это моё — снять"
                    : "Это моё"}
              </button>
            </article>
          );
        })}
      </section>

      <section className="card participant-breakdown">
        <div>
          <p className="eyebrow">Текущая доля</p>
          <h2>
            {formatMoney(
              props.participantPreview?.totalAmount ?? 0,
              props.room.currency,
            )}
          </h2>
        </div>

        {props.participantPreview && (
          <div className="breakdown-grid">
            <BreakdownValue
              label="Блюда"
              value={props.participantPreview.baseAmount}
              room={props.room}
            />
            <BreakdownValue
              label="Сервис"
              value={props.participantPreview.serviceShare}
              room={props.room}
            />
            <BreakdownValue
              label="Чаевые"
              value={props.participantPreview.tipShare}
              room={props.room}
            />
            <BreakdownValue
              label="Скидка"
              value={-props.participantPreview.discountShare}
              room={props.room}
            />
          </div>
        )}

        <p className="muted">
          Вес влияет только на деление конкретного общего блюда. Значение 2
          означает вдвое большую долю относительно веса 1.
        </p>
      </section>

      <ParticipantFooter onRefresh={props.onRefresh} onLeave={props.onLeave} />
    </>
  );
}

function StatusPanel({
  room,
  isAdmin,
  loading,
  unassignedCount,
  onOpen,
  onFinalize,
  onReopen,
}: {
  room: Room;
  isAdmin: boolean;
  loading: boolean;
  unassignedCount: number;
  onOpen: () => void;
  onFinalize: () => void;
  onReopen: () => void;
}) {
  const labels = {
    draft: "Подготовка чека",
    claiming: "Участники выбирают блюда",
    finalized: "Распределение завершено",
  } as const;

  return (
    <section className="card status-panel">
      <div>
        <p className="eyebrow">Статус комнаты</p>
        <h2>{labels[room.status]}</h2>
        {unassignedCount > 0 && room.status !== "finalized" && (
          <p className="status-warning">
            Нераспределённых позиций: {unassignedCount}
          </p>
        )}
      </div>

      {isAdmin && (
        <div className="actions">
          {room.status === "draft" && (
            <button disabled={loading} onClick={onOpen}>
              Открыть распределение
            </button>
          )}
          {room.status === "claiming" && (
            <button
              disabled={loading || unassignedCount > 0}
              onClick={onFinalize}
            >
              Завершить распределение
            </button>
          )}
          {room.status === "finalized" && (
            <button className="secondary" disabled={loading} onClick={onReopen}>
              Вернуть к редактированию
            </button>
          )}
        </div>
      )}
    </section>
  );
}

function CalculationResult({
  calculation,
  room,
  focusParticipantId,
}: {
  calculation: CalculateResponse;
  room: Room;
  focusParticipantId?: string;
}) {
  const visibleResults = focusParticipantId
    ? calculation.results.filter(
        (result) => result.participant_id === focusParticipantId,
      )
    : calculation.results;

  const visibleDebts = focusParticipantId
    ? calculation.debts.filter(
        (debt) =>
          debt.from_participant_id === focusParticipantId ||
          debt.to_participant_id === focusParticipantId,
      )
    : calculation.debts;

  return (
    <>
      <div className="summary-grid">
        <p>
          Позиции:{" "}
          <strong>{formatMoney(calculation.subtotal, room.currency)}</strong>
        </p>
        <p>
          Рассчитано:{" "}
          <strong>
            {formatMoney(calculation.calculated_total, room.currency)}
          </strong>
        </p>
        {room.expected_total > 0 && (
          <p>
            На чеке:{" "}
            <strong>{formatMoney(room.expected_total, room.currency)}</strong>
          </p>
        )}
      </div>

      {room.expected_total > 0 &&
        (calculation.matches_expected_total ? (
          <p className="success">Сумма совпадает с итогом на чеке.</p>
        ) : (
          <p className="error">
            Расхождение: {formatMoney(calculation.difference, room.currency)}
          </p>
        ))}

      <div className="table-scroll">
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
            {visibleResults.map((result) => (
              <tr key={result.participant_id}>
                <td>{result.name}</td>
                <td>{formatMoney(result.base_amount, room.currency)}</td>
                <td>{formatMoney(result.service_share, room.currency)}</td>
                <td>{formatMoney(result.tip_share, room.currency)}</td>
                <td>−{formatMoney(result.discount_share, room.currency)}</td>
                <td>
                  <strong>
                    {formatMoney(result.total_amount, room.currency)}
                  </strong>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="debt-list">
        <h3>Кто кому должен</h3>
        {visibleDebts.length === 0 ? (
          <p className="muted">Переводы не требуются.</p>
        ) : (
          visibleDebts.map((debt) => (
            <p key={`${debt.from_participant_id}:${debt.to_participant_id}`}>
              <strong>{debt.from_name}</strong> должен(на){" "}
              <strong>{debt.to_name}</strong>:{" "}
              {formatMoney(debt.amount, room.currency)}
            </p>
          ))
        )}
      </div>
    </>
  );
}

function RoomHeader({
  room,
  role,
  lastUpdatedAt,
}: {
  room: Room;
  role: string;
  lastUpdatedAt: Date | null;
}) {
  return (
    <header className="room-header">
      <div>
        <p className="eyebrow">{role}</p>
        <h1>{room.title}</h1>
        <p className="muted">
          Комната <code>{room.id}</code>
        </p>
      </div>
      <div className="live-indicator">
        <span />
        <div>
          <strong>Автообновление</strong>
          <small>
            {lastUpdatedAt
              ? lastUpdatedAt.toLocaleTimeString("ru-RU")
              : "загрузка"}
          </small>
        </div>
      </div>
    </header>
  );
}

function MoneyInput({
  label,
  value,
  onChange,
  disabled,
  positive = false,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  disabled: boolean;
  positive?: boolean;
}) {
  return (
    <label>
      {label}
      <input
        type="number"
        min={positive ? "0.01" : "0"}
        step="0.01"
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      />
    </label>
  );
}

function BreakdownValue({
  label,
  value,
  room,
}: {
  label: string;
  value: number;
  room: Room;
}) {
  return (
    <span>
      {label}
      <strong>{formatMoney(value, room.currency)}</strong>
    </span>
  );
}

function ParticipantFooter({
  onRefresh,
  onLeave,
}: {
  onRefresh: () => void;
  onLeave: () => void;
}) {
  return (
    <div className="participant-footer">
      <button type="button" className="secondary" onClick={onRefresh}>
        Обновить сейчас
      </button>
      <button type="button" className="link-button" onClick={onLeave}>
        Выйти из участника
      </button>
    </div>
  );
}

function translateError(message: string): string {
  const translations: Record<string, string> = {
    "room is finalized": "Комната уже завершена и заблокирована.",
    "room is not open for selections": "Организатор ещё не открыл выбор блюд.",
    "all items must have at least one participant":
      "Каждая позиция должна быть назначена хотя бы одному участнику.",
    "payer is required before finalization":
      "Перед завершением выберите человека, который оплатил чек.",
    "calculated total does not match expected total":
      "Рассчитанная сумма не совпадает с итогом на чеке.",
  };

  return translations[message] ?? message;
}
