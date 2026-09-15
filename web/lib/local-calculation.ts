import type { ItemAssignment, Participant, ReceiptItem, Room } from "./api";

export type ParticipantPreview = {
  baseAmount: number;
  serviceShare: number;
  tipShare: number;
  discountShare: number;
  totalAmount: number;
};

type WeightedRecipient = {
  participantId: string;
  weight: number;
};

export function calculateParticipantPreview(
  room: Room,
  participants: Participant[],
  items: ReceiptItem[],
  assignments: ItemAssignment[],
  participantId: string,
): ParticipantPreview {
  const base = new Map<string, number>();

  for (const participant of participants) {
    base.set(participant.id, 0);
  }

  for (const item of items) {
    const recipients = assignments
      .filter((assignment) => assignment.item_id === item.id)
      .map((assignment) => ({
        participantId: assignment.participant_id,
        weight: assignment.weight,
      }));

    if (recipients.length === 0) {
      continue;
    }

    const shares = splitByWeights(item.total, recipients);

    for (const [id, amount] of shares) {
      base.set(id, (base.get(id) ?? 0) + amount);
    }
  }

  const serviceShares = splitProportionally(room.service_fee, base);
  const tipShares = splitProportionally(room.tip_amount, base);

  const gross = new Map<string, number>();
  for (const [id, amount] of base) {
    gross.set(
      id,
      amount + (serviceShares.get(id) ?? 0) + (tipShares.get(id) ?? 0),
    );
  }

  const discountShares =
    room.discount_mode === "equal"
      ? splitEquallyWithCapacity(room.discount, gross)
      : splitProportionally(room.discount, base);

  const baseAmount = base.get(participantId) ?? 0;
  const serviceShare = serviceShares.get(participantId) ?? 0;
  const tipShare = tipShares.get(participantId) ?? 0;
  const discountShare = discountShares.get(participantId) ?? 0;

  return {
    baseAmount,
    serviceShare,
    tipShare,
    discountShare,
    totalAmount: baseAmount + serviceShare + tipShare - discountShare,
  };
}

function splitByWeights(
  total: number,
  recipients: WeightedRecipient[],
): Map<string, number> {
  const result = new Map<string, number>();

  const totalWeight = recipients.reduce(
    (sum, recipient) => sum + recipient.weight,
    0,
  );

  if (totalWeight <= 0) {
    return result;
  }

  const remainders: Remainder[] = [];
  let distributed = 0;

  for (const recipient of recipients) {
    const numerator = total * recipient.weight;
    const amount = Math.floor(numerator / totalWeight);
    const remainder = numerator % totalWeight;

    result.set(
      recipient.participantId,
      (result.get(recipient.participantId) ?? 0) + amount,
    );
    distributed += amount;

    remainders.push({
      participantId: recipient.participantId,
      value: remainder,
    });
  }

  distributeRemainder(total - distributed, remainders, result);

  return result;
}

function splitProportionally(
  total: number,
  base: Map<string, number>,
): Map<string, number> {
  const result = zeroShares(base);

  if (total === 0) {
    return result;
  }

  const baseTotal = Array.from(base.values()).reduce(
    (sum, amount) => sum + amount,
    0,
  );

  if (baseTotal <= 0) {
    return result;
  }

  const remainders: Remainder[] = [];
  let distributed = 0;

  for (const [participantId, amount] of base) {
    const numerator = total * amount;
    const share = Math.floor(numerator / baseTotal);
    const remainder = numerator % baseTotal;

    result.set(participantId, share);
    distributed += share;
    remainders.push({
      participantId,
      value: remainder,
    });
  }

  distributeRemainder(total - distributed, remainders, result);

  return result;
}

function splitEquallyWithCapacity(
  total: number,
  capacity: Map<string, number>,
): Map<string, number> {
  const result = zeroShares(capacity);

  if (total === 0) {
    return result;
  }

  const remainingCapacity = new Map(capacity);

  let active = Array.from(capacity.entries())
    .filter(([, amount]) => amount > 0)
    .map(([participantId]) => participantId)
    .sort();

  let remaining = total;

  while (remaining > 0 && active.length > 0) {
    let share = Math.floor(remaining / active.length);
    let extra = remaining % active.length;

    if (share === 0) {
      share = 1;
      extra = 0;
    }

    let distributedThisRound = 0;
    const nextActive: string[] = [];

    active.forEach((participantId, index) => {
      let desired = share + (index < extra ? 1 : 0);

      desired = Math.min(desired, remaining - distributedThisRound);

      const available = remainingCapacity.get(participantId) ?? 0;
      const actual = Math.min(desired, available);

      result.set(participantId, (result.get(participantId) ?? 0) + actual);
      remainingCapacity.set(participantId, available - actual);
      distributedThisRound += actual;

      if (available - actual > 0) {
        nextActive.push(participantId);
      }
    });

    if (distributedThisRound === 0) {
      break;
    }

    remaining -= distributedThisRound;
    active = nextActive;
  }

  return result;
}

type Remainder = {
  participantId: string;
  value: number;
};

function zeroShares(source: Map<string, number>): Map<string, number> {
  const result = new Map<string, number>();

  for (const participantId of source.keys()) {
    result.set(participantId, 0);
  }

  return result;
}

function distributeRemainder(
  left: number,
  remainders: Remainder[],
  result: Map<string, number>,
): void {
  if (left <= 0 || remainders.length === 0) {
    return;
  }

  remainders.sort((a, b) => {
    if (a.value === b.value) {
      return a.participantId.localeCompare(b.participantId);
    }

    return b.value - a.value;
  });

  for (let index = 0; index < left; index += 1) {
    const recipient = remainders[index % remainders.length];

    result.set(
      recipient.participantId,
      (result.get(recipient.participantId) ?? 0) + 1,
    );
  }
}
