const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type RoomStatus = "draft" | "claiming" | "finalized";

export type DiscountMode = "proportional" | "equal";

export type Room = {
  id: string;
  title: string;
  currency: string;
  service_fee: number;
  tip_amount: number;
  discount: number;
  discount_mode: DiscountMode;
  expected_total: number;
  payer_participant_id: string;
  status: RoomStatus;
  finalized_at: string | null;
};

export type Participant = {
  id: string;
  room_id: string;
  name: string;
  claimed: boolean;
};

export type ReceiptItem = {
  id: string;
  room_id: string;
  name: string;
  quantity: number;
  unit_price: number;
  total: number;
};

export type ItemAssignment = {
  item_id: string;
  participant_id: string;
  weight: number;
};

export type ParticipantResult = {
  participant_id: string;
  name: string;
  base_amount: number;
  service_share: number;
  tip_share: number;
  discount_share: number;
  total_amount: number;
};

export type Debt = {
  from_participant_id: string;
  from_name: string;
  to_participant_id: string;
  to_name: string;
  amount: number;
};

export type RoomDetails = {
  room: Room;
  participants: Participant[];
  items: ReceiptItem[];
  assignments: ItemAssignment[];
  subtotal: number;
  unassigned_item_ids: string[];
};

export type CreateRoomResponse = {
  room: Room;
  admin_token: string;
};

export type JoinRoomResponse = {
  participant: Participant;
  participant_token: string;
};

export type CalculateResponse = {
  room: Room;
  results: ParticipantResult[];
  debts: Debt[];
  subtotal: number;
  calculated_total: number;
  difference: number;
  matches_expected_total: boolean;
};

type APIErrorPayload = {
  error?: string;
  message?: string;
};

type RequestOptions = RequestInit & {
  adminToken?: string;
  participantToken?: string;
};

async function request<T>(path: string, options?: RequestOptions): Promise<T> {
  const { adminToken, participantToken, ...fetchOptions } = options ?? {};

  const headers = new Headers(fetchOptions.headers);

  if (fetchOptions.body !== undefined) {
    headers.set("Content-Type", "application/json");
  }

  if (adminToken) {
    headers.set("X-Admin-Token", adminToken);
  }

  if (participantToken) {
    headers.set("X-Participant-Token", participantToken);
  }

  const response = await fetch(`${API_URL}${path}`, {
    ...fetchOptions,
    headers,
    cache: "no-store",
  });

  if (response.status === 204) {
    return undefined as T;
  }

  const data = (await response.json().catch(() => null)) as
    | APIErrorPayload
    | T
    | null;

  if (!response.ok) {
    const errorData = data as APIErrorPayload | null;

    throw new Error(
      errorData?.error ??
        errorData?.message ??
        `Ошибка запроса: ${response.status}`,
    );
  }

  return data as T;
}

function id(value: string): string {
  return encodeURIComponent(value);
}

export function createRoom(payload: {
  title: string;
  currency: string;
  expected_total?: number;
  discount_mode?: DiscountMode;
}): Promise<CreateRoomResponse> {
  return request<CreateRoomResponse>("/rooms", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function getRoom(roomId: string): Promise<RoomDetails> {
  return request<RoomDetails>(`/rooms/${id(roomId)}`);
}

export function updateRoom(
  roomId: string,
  adminToken: string,
  payload: Partial<{
    title: string;
    currency: string;
    service_fee: number;
    tip_amount: number;
    discount: number;
    discount_mode: DiscountMode;
    expected_total: number;
    payer_participant_id: string;
  }>,
): Promise<Room> {
  return request<Room>(`/rooms/${id(roomId)}`, {
    method: "PATCH",
    adminToken,
    body: JSON.stringify(payload),
  });
}

export function joinRoom(
  roomId: string,
  payload: { name: string },
): Promise<JoinRoomResponse> {
  return request<JoinRoomResponse>(`/rooms/${id(roomId)}/join`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function addParticipant(
  roomId: string,
  adminToken: string,
  payload: { name: string },
): Promise<Participant> {
  return request<Participant>(`/rooms/${id(roomId)}/participants`, {
    method: "POST",
    adminToken,
    body: JSON.stringify(payload),
  });
}

export function updateParticipant(
  roomId: string,
  participantId: string,
  adminToken: string,
  payload: { name: string },
): Promise<Participant> {
  return request<Participant>(
    `/rooms/${id(roomId)}/participants/${id(participantId)}`,
    {
      method: "PATCH",
      adminToken,
      body: JSON.stringify(payload),
    },
  );
}

export function deleteParticipant(
  roomId: string,
  participantId: string,
  adminToken: string,
): Promise<void> {
  return request<void>(
    `/rooms/${id(roomId)}/participants/${id(participantId)}`,
    {
      method: "DELETE",
      adminToken,
    },
  );
}

export function addItem(
  roomId: string,
  adminToken: string,
  payload: {
    name: string;
    quantity: number;
    unit_price: number;
  },
): Promise<ReceiptItem> {
  return request<ReceiptItem>(`/rooms/${id(roomId)}/items`, {
    method: "POST",
    adminToken,
    body: JSON.stringify(payload),
  });
}

export function updateItem(
  roomId: string,
  itemId: string,
  adminToken: string,
  payload: Partial<{
    name: string;
    quantity: number;
    unit_price: number;
  }>,
): Promise<ReceiptItem> {
  return request<ReceiptItem>(`/rooms/${id(roomId)}/items/${id(itemId)}`, {
    method: "PATCH",
    adminToken,
    body: JSON.stringify(payload),
  });
}

export function deleteItem(
  roomId: string,
  itemId: string,
  adminToken: string,
): Promise<void> {
  return request<void>(`/rooms/${id(roomId)}/items/${id(itemId)}`, {
    method: "DELETE",
    adminToken,
  });
}

export function addAssignment(
  roomId: string,
  adminToken: string,
  payload: {
    item_id: string;
    participant_id: string;
    weight: number;
  },
): Promise<ItemAssignment> {
  return request<ItemAssignment>(`/rooms/${id(roomId)}/assignments`, {
    method: "POST",
    adminToken,
    body: JSON.stringify(payload),
  });
}

export function deleteAssignment(
  roomId: string,
  itemId: string,
  participantId: string,
  adminToken: string,
): Promise<void> {
  return request<void>(
    `/rooms/${id(roomId)}/assignments/${id(itemId)}/${id(participantId)}`,
    {
      method: "DELETE",
      adminToken,
    },
  );
}

export function selectItem(
  roomId: string,
  itemId: string,
  participantToken: string,
  weight: number,
): Promise<ItemAssignment> {
  return request<ItemAssignment>(
    `/rooms/${id(roomId)}/selections/${id(itemId)}`,
    {
      method: "PUT",
      participantToken,
      body: JSON.stringify({ weight }),
    },
  );
}

export function unselectItem(
  roomId: string,
  itemId: string,
  participantToken: string,
): Promise<void> {
  return request<void>(`/rooms/${id(roomId)}/selections/${id(itemId)}`, {
    method: "DELETE",
    participantToken,
  });
}

export function openRoomSelections(
  roomId: string,
  adminToken: string,
): Promise<Room> {
  return request<Room>(`/rooms/${id(roomId)}/open`, {
    method: "POST",
    adminToken,
  });
}

export function finalizeRoom(
  roomId: string,
  adminToken: string,
): Promise<CalculateResponse> {
  return request<CalculateResponse>(`/rooms/${id(roomId)}/finalize`, {
    method: "POST",
    adminToken,
  });
}

export function reopenRoom(roomId: string, adminToken: string): Promise<Room> {
  return request<Room>(`/rooms/${id(roomId)}/reopen`, {
    method: "POST",
    adminToken,
  });
}

export function calculateRoom(roomId: string): Promise<CalculateResponse> {
  return request<CalculateResponse>(`/rooms/${id(roomId)}/calculate`, {
    method: "POST",
  });
}
