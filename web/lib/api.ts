const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export type Room = {
  id: string;
  title: string;
  currency: string;
  service_fee: number;
  tip_amount: number;
  discount: number;
  total_amount: number;
};

export type Participant = {
  id: string;
  room_id: string;
  name: string;
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

export type RoomDetails = {
  room: Room;
  participants: Participant[];
  items: ReceiptItem[];
  assignments: ItemAssignment[];
};

export type CalculateResponse = {
  room: Room;
  results: ParticipantResult[];
  calculated_total: number;
};

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(options?.headers ?? {}),
    },
  });

  const data = await response.json().catch(() => null);

  if (!response.ok) {
    const message = data?.message || "Request failed";
    throw new Error(message);
  }

  return data as T;
}

export function createRoom(payload: {
  title: string;
  currency: string;
  service_fee: number;
  tip_amount: number;
  discount: number;
}) {
  return request<Room>("/rooms", {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function getRoom(roomId: string) {
  return request<RoomDetails>(`/rooms/${roomId}`);
}

export function updateRoom(
  roomId: string,
  payload: Partial<{
    title: string;
    currency: string;
    service_fee: number;
    tip_amount: number;
    discount: number;
    total_amount: number;
  }>,
) {
  return request<Room>(`/rooms/${roomId}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
}

export function addParticipant(roomId: string, payload: { name: string }) {
  return request<Participant>(`/rooms/${roomId}/paricipants`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function addItem(
  roomId: string,
  payload: { name: string; quantity: number; unit_price: number },
) {
  return request<ReceiptItem>(`/rooms/${roomId}/items`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function addAssignment(
  roomId: string,
  payload: {
    item_id: string;
    participant_id: string;
    weight: number;
  },
) {
  return request<ItemAssignment>(`/rooms/${roomId}/assignments`, {
    method: "POST",
    body: JSON.stringify(payload),
  });
}

export function calculateRoom(roomId: string) {
  return request<CalculateResponse>(`/rooms/${roomId}/calculate`, {
    method: "POST",
  });
}
