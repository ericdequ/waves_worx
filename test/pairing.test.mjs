import { test } from 'node:test';
import assert from 'node:assert/strict';

import { createPairing, PAIRING_STATE, createLoopbackBus, createLoopbackChannel } from '../src/index.js';

// Fixed nonces so the derived session is deterministic.
const initiator = (code) => createPairing({ roomCode: code, role: 'initiator', nonce: () => 'AAAA' });
const responder = (code) => createPairing({ roomCode: code, role: 'responder', nonce: () => 'BBBB' });

test('pairing: full offer → answer → confirm reaches PAIRED on both sides', () => {
  const a = initiator('ROOM42');
  const b = responder('ROOM42');

  const offer = a.start();
  assert.equal(a.state, PAIRING_STATE.OFFERED);

  const bAfterOffer = b.receive(offer);
  assert.equal(b.state, PAIRING_STATE.ANSWERED);

  const aAfterAnswer = a.receive(bAfterOffer.send);
  assert.equal(a.state, PAIRING_STATE.PAIRED);

  b.receive(aAfterAnswer.send);
  assert.equal(b.state, PAIRING_STATE.PAIRED);

  // Both independently derived the SAME session id.
  assert.ok(a.session);
  assert.equal(a.session, b.session);
});

test('pairing: a room-code mismatch fails closed', () => {
  const a = initiator('ROOM42');
  const b = responder('OTHER9');
  const res = b.receive(a.start());
  assert.equal(b.state, PAIRING_STATE.FAILED);
  assert.equal(res.reason, 'room-code-mismatch');
});

test('pairing: a tampered confirm session is rejected', () => {
  const a = initiator('ROOM42');
  const b = responder('ROOM42');
  const offer = a.start();
  const answer = b.receive(offer).send;
  const confirm = a.receive(answer).send;
  const tampered = { ...confirm, session: 'deadbeefdeadbeef' };
  const res = b.receive(tampered);
  assert.equal(b.state, PAIRING_STATE.FAILED);
  assert.equal(res.reason, 'session-mismatch');
});

test('pairing: drives end-to-end over a loopback channel', () => {
  const bus = createLoopbackBus();
  const chanA = createLoopbackChannel({ id: 'A', bus });
  const chanB = createLoopbackChannel({ id: 'B', bus });
  const a = initiator('ROOM42');
  const b = responder('ROOM42');

  const wire = (chan, endpoint) =>
    chan.subscribe((bytes) => {
      const msg = JSON.parse(new TextDecoder().decode(bytes));
      const { send } = endpoint.receive(msg);
      if (send) chan.send(new TextEncoder().encode(JSON.stringify(send)));
    });

  wire(chanA, a);
  wire(chanB, b);
  chanA.send(new TextEncoder().encode(JSON.stringify(a.start())));

  assert.equal(a.state, PAIRING_STATE.PAIRED);
  assert.equal(b.state, PAIRING_STATE.PAIRED);
  assert.equal(a.session, b.session);
});
