import { pbkdf2Async } from '@noble/hashes/pbkdf2.js';
import { sha512 } from '@noble/hashes/sha2.js';

const textEncoder = new TextEncoder();

export async function deriveClientHash(passwordText: string, saltText: string, rounds: number) {
  const normalizedPassword = passwordText ?? '';
  const normalizedSalt = saltText ?? '';
  const normalizedRounds = Number.isFinite(rounds) ? Math.max(1, Math.trunc(rounds)) : 20000;
  const passwordBytes = textEncoder.encode(normalizedPassword);
  const saltBytes = textEncoder.encode(normalizedSalt);

  if (globalThis.crypto?.subtle) {
    try {
      const baseKey = await globalThis.crypto.subtle.importKey(
        'raw',
        passwordBytes,
        { name: 'PBKDF2' },
        false,
        ['deriveBits']
      );

      const derivedBits = await globalThis.crypto.subtle.deriveBits(
        {
          name: 'PBKDF2',
          hash: 'SHA-512',
          salt: saltBytes,
          iterations: normalizedRounds,
        },
        baseKey,
        64 * 8
      );

      return bytesToHex(new Uint8Array(derivedBits));
    } catch {
      // Fall back to a pure JS implementation for insecure HTTP contexts or limited browsers.
    }
  }

  const derivedBytes = await pbkdf2Async(sha512, passwordBytes, saltBytes, {
    c: normalizedRounds,
    dkLen: 64,
  });
  return bytesToHex(derivedBytes);
}

function bytesToHex(bytes: Uint8Array) {
  return Array.from(bytes, value => value.toString(16).padStart(2, '0')).join('');
}
