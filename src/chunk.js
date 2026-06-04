// =============================================================================
// waves_worx — chunk: send a payload over a low-bandwidth, lossy channel
// =============================================================================
// QR codes, ultrasonic bursts, and light flashes can only carry a few bytes at
// a time and drop frames. chunk() splits a payload into sequenced, CRC-tagged
// frames; a Reassembler collects them back in any order, tolerates duplicates,
// reports what's still missing, and verifies integrity on completion.
//
// This is the no-cloud transport core: it does not care WHICH physical channel
// carries the frames — pair it with any channel from ./channel.js.
// =============================================================================

import { crc32,toBytes } from './frame.js';

export const DEFAULT_CHUNK_BYTES = 180; // fits a comfortably-scannable QR

/**
 * Split a payload into frames. Each frame:
 *   { id, index, total, bytes, crc }
 * `id` ties a frame set together so a receiver can run multiple transfers.
 *
 * @param {Uint8Array|string} payload
 * @param {{id?: string, size?: number}} [opts]
 * @returns {Array<{id:string,index:number,total:number,bytes:Uint8Array,crc:number}>}
 */
export function chunk(payload, { id = 't', size = DEFAULT_CHUNK_BYTES } = {}) {
  const bytes = toBytes(payload);
  const step = Math.max(1, Math.floor(size));
  const total = Math.max(1, Math.ceil(bytes.length / step));
  const frames = [];
  for (let index = 0; index < total; index++) {
    const slice = bytes.subarray(index * step, index * step + step);
    frames.push(Object.freeze({ id: String(id), index, total, bytes: new Uint8Array(slice), crc: crc32(slice) }));
  }
  return frames;
}

/**
 * Stateful collector. Feed it frames (any order, duplicates fine); ask whether
 * the transfer is complete and pull the reassembled payload.
 */
export class Reassembler {
  constructor() {
    this._id = null;
    this._total = 0;
    this._parts = new Map(); // index -> Uint8Array (CRC-verified)
  }

  /**
   * Add a frame. Returns a status snapshot. Frames whose CRC fails, or that
   * belong to a different transfer id than the one in progress, are rejected.
   * @returns {{accepted:boolean, complete:boolean, received:number, total:number, reason?:string}}
   */
  add(frame) {
    if (!frame || typeof frame.index !== 'number' || typeof frame.total !== 'number') {
      return this._status(false, 'malformed-frame');
    }
    if (this._id === null) {
      this._id = String(frame.id ?? 't');
      this._total = frame.total;
    } else if (String(frame.id ?? 't') !== this._id) {
      return this._status(false, 'wrong-transfer');
    }
    const bytes = toBytes(frame.bytes);
    if (crc32(bytes) !== (frame.crc >>> 0)) {
      return this._status(false, 'crc-mismatch');
    }
    this._parts.set(frame.index, new Uint8Array(bytes));
    return this._status(true);
  }

  get complete() {
    return this._total > 0 && this._parts.size === this._total;
  }

  /** Indices not yet received. */
  missing() {
    const out = [];
    for (let i = 0; i < this._total; i++) if (!this._parts.has(i)) out.push(i);
    return out;
  }

  /** The reassembled payload, or null if incomplete. */
  bytes() {
    if (!this.complete) return null;
    const ordered = [];
    let length = 0;
    for (let i = 0; i < this._total; i++) {
      const part = this._parts.get(i);
      ordered.push(part);
      length += part.length;
    }
    const out = new Uint8Array(length);
    let offset = 0;
    for (const part of ordered) {
      out.set(part, offset);
      offset += part.length;
    }
    return out;
  }

  _status(accepted, reason) {
    return {
      accepted,
      complete: this.complete,
      received: this._parts.size,
      total: this._total,
      ...(reason ? { reason } : {}),
    };
  }
}
