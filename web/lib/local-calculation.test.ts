import { describe, expect, it } from "vitest";

import type { ItemAssignment, Participant, ReceiptItem, Room } from "./api";
import { calculateParticipantPreview } from "./local-calculation";

function makeRoom(overrides: Partial<Room> = {}): Room {
  return {
    id: "room-1",
    title: "Dinner",
    currency: "EUR",
    service_fee: 0,
    tip_amount: 0,
    discount: 0,
    discount_mode: "proportional",
    expected_total: 0,
    payer_participant_id: "",
    status: "claiming",
    finalized_at: null,
    ...overrides,
  };
}

function makeParticipants(...ids: string[]): Participant[] {
  return ids.map((id) => ({
    id,
    room_id: "room-1",
    name: id,
    claimed: true,
  }));
}

function makeItem(id: string, total: number): ReceiptItem {
  return {
    id,
    room_id: "room-1",
    name: id,
    quantity: 1,
    unit_price: total,
    total,
  };
}

function makeAssignment(
  itemId: string,
  participantId: string,
  weight = 1,
): ItemAssignment {
  return { item_id: itemId, participant_id: participantId, weight };
}

describe("calculateParticipantPreview", () => {
  it("splits a shared item equally between two participants", () => {
    const room = makeRoom();
    const participants = makeParticipants("p1", "p2");
    const items = [makeItem("i1", 1000)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i1", "p2"),
    ];

    const preview = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p1",
    );

    expect(preview.baseAmount).toBe(500);
    expect(preview.totalAmount).toBe(500);
  });

  it("gives a participant with weight 2 double the share of weight 1", () => {
    const room = makeRoom();
    const participants = makeParticipants("p1", "p2");
    const items = [makeItem("i1", 900)];
    const assignments = [
      makeAssignment("i1", "p1", 2),
      makeAssignment("i1", "p2", 1),
    ];

    const heavy = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p1",
    );
    const light = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p2",
    );

    expect(heavy.baseAmount).toBe(600);
    expect(light.baseAmount).toBe(300);
    expect(heavy.baseAmount).toBe(2 * light.baseAmount);
  });

  it("splits service fee and tip proportionally to the base amount", () => {
    const room = makeRoom({ service_fee: 100, tip_amount: 200 });
    const participants = makeParticipants("p1", "p2");
    const items = [makeItem("i1", 900), makeItem("i2", 100)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i2", "p2"),
    ];

    const first = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p1",
    );
    const second = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p2",
    );

    expect(first.serviceShare).toBe(90);
    expect(second.serviceShare).toBe(10);
    expect(first.tipShare).toBe(180);
    expect(second.tipShare).toBe(20);
    expect(first.totalAmount).toBe(1170);
    expect(second.totalAmount).toBe(130);
  });

  it("gives a larger base a larger proportional discount share", () => {
    const room = makeRoom({ discount: 200, discount_mode: "proportional" });
    const participants = makeParticipants("p1", "p2");
    const items = [makeItem("i1", 900), makeItem("i2", 100)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i2", "p2"),
    ];

    const first = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p1",
    );
    const second = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p2",
    );

    expect(first.discountShare).toBe(180);
    expect(second.discountShare).toBe(20);
    expect(first.discountShare).toBeGreaterThan(second.discountShare);
  });

  it("splits an equal discount evenly between participants", () => {
    const room = makeRoom({ discount: 200, discount_mode: "equal" });
    const participants = makeParticipants("p1", "p2");
    const items = [makeItem("i1", 900), makeItem("i2", 100)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i2", "p2"),
    ];

    const first = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p1",
    );
    const second = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p2",
    );

    expect(first.discountShare).toBe(100);
    expect(second.discountShare).toBe(100);
  });

  it("never lets a small participant go negative with an equal discount", () => {
    const room = makeRoom({ discount: 200, discount_mode: "equal" });
    const participants = makeParticipants("p1", "p2");
    const items = [makeItem("i1", 990), makeItem("i2", 10)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i2", "p2"),
    ];

    const small = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p2",
    );

    expect(small.discountShare).toBe(10);
    expect(small.totalAmount).toBe(0);
    expect(small.totalAmount).toBeGreaterThanOrEqual(0);
  });

  it("redistributes the equal discount remainder", () => {
    const room = makeRoom({ discount: 100, discount_mode: "equal" });
    const participants = makeParticipants("p1", "p2", "p3");
    const items = [makeItem("i1", 1000)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i1", "p2"),
      makeAssignment("i1", "p3"),
    ];

    const shares = ["p1", "p2", "p3"].map(
      (id) =>
        calculateParticipantPreview(room, participants, items, assignments, id)
          .discountShare,
    );

    expect(shares.reduce((sum, share) => sum + share, 0)).toBe(100);
    expect(shares).toEqual([34, 33, 33]);
  });

  it("keeps the sum of all shares equal to the bill total", () => {
    const room = makeRoom({
      service_fee: 100,
      tip_amount: 100,
      discount: 100,
      discount_mode: "proportional",
    });
    const participants = makeParticipants("p1", "p2", "p3");
    const items = [makeItem("i1", 1000)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i1", "p2"),
      makeAssignment("i1", "p3"),
    ];

    const total = ["p1", "p2", "p3"].reduce(
      (sum, id) =>
        sum +
        calculateParticipantPreview(room, participants, items, assignments, id)
          .totalAmount,
      0,
    );

    expect(total).toBe(1100);
  });

  it("preserves the total when splitting an awkward amount", () => {
    const room = makeRoom();
    const participants = makeParticipants("p1", "p2", "p3");
    const items = [makeItem("i1", 100)];
    const assignments = [
      makeAssignment("i1", "p1"),
      makeAssignment("i1", "p2"),
      makeAssignment("i1", "p3"),
    ];

    const total = ["p1", "p2", "p3"].reduce(
      (sum, id) =>
        sum +
        calculateParticipantPreview(room, participants, items, assignments, id)
          .totalAmount,
      0,
    );

    expect(total).toBe(100);
  });

  it("returns zero shares for a participant without assignments", () => {
    const room = makeRoom();
    const participants = makeParticipants("p1", "p2");
    const items = [makeItem("i1", 1000)];
    const assignments = [makeAssignment("i1", "p1")];

    const preview = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p2",
    );

    expect(preview.baseAmount).toBe(0);
    expect(preview.totalAmount).toBe(0);
  });

  it("ignores items without assignments", () => {
    const room = makeRoom();
    const participants = makeParticipants("p1");
    const items = [makeItem("i1", 1000), makeItem("i2", 500)];
    const assignments = [makeAssignment("i1", "p1")];

    const preview = calculateParticipantPreview(
      room,
      participants,
      items,
      assignments,
      "p1",
    );

    expect(preview.baseAmount).toBe(1000);
  });
});
