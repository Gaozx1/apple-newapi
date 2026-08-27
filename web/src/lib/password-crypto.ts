/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

/**
 * Password-in-transit encryption using RSA-OAEP (Web Crypto API).
 *
 * The backend exposes a PEM-encoded RSA public key via `/api/status`
 * (`login_rsa_public_key`). The frontend imports it, encrypts the password
 * with RSA-OAEP/SHA-256, and sends the Base64 ciphertext. The server
 * decrypts with the matching private key before bcrypt comparison.
 *
 * If no public key is available (older backend or key not initialized),
 * `encryptPassword` returns the plaintext unchanged so the flow degrades
 * gracefully — the server also falls back to plaintext in that case.
 */

// Cache the imported CryptoKey so we don't re-import the PEM on every call.
let cachedPublicKey: CryptoKey | null = null
let cachedPublicKeyPEM = ''

/**
 * Convert a PEM string to a DER ArrayBuffer for subtle crypto import.
 */
function pemToDER(pem: string): ArrayBuffer {
  const trimmed = pem.replace(/-----[A-Z ]+-----/g, '').replace(/\s+/g, '')
  const binaryString = atob(trimmed)
  const bytes = new Uint8Array(binaryString.length)
  for (let i = 0; i < binaryString.length; i++) {
    bytes[i] = binaryString.charCodeAt(i)
  }
  return bytes.buffer
}

/**
 * Import a PEM public key into a Web Crypto CryptoKey for RSA-OAEP/SHA-256.
 */
async function importPublicKey(pem: string): Promise<CryptoKey | null> {
  if (!pem || typeof crypto === 'undefined' || !crypto.subtle) return null
  try {
    const der = pemToDER(pem)
    return await crypto.subtle.importKey(
      'spki',
      der,
      { name: 'RSA-OAEP', hash: 'SHA-256' },
      false,
      ['encrypt']
    )
  } catch {
    return null
  }
}

/**
 * Encrypt a password with the login RSA public key.
 *
 * @param password The plaintext password to encrypt.
 * @param publicKeyPEM The PEM-encoded RSA public key from `/api/status`.
 * @returns Base64-encoded ciphertext, or the original plaintext if
 *          encryption is unavailable (no key, no Web Crypto, or error).
 */
export async function encryptPassword(
  password: string,
  publicKeyPEM?: string
): Promise<string> {
  if (!password || !publicKeyPEM) return password
  if (typeof crypto === 'undefined' || !crypto.subtle) return password

  // Reuse cached key if the PEM hasn't changed.
  let key = cachedPublicKey
  if (key === null || publicKeyPEM !== cachedPublicKeyPEM) {
    key = await importPublicKey(publicKeyPEM)
    if (key === null) return password
    cachedPublicKey = key
    cachedPublicKeyPEM = publicKeyPEM
  }

  try {
    const encoded = new TextEncoder().encode(password)
    const ciphertext = await crypto.subtle.encrypt(
      { name: 'RSA-OAEP' },
      key,
      encoded
    )
    // Convert ArrayBuffer to Base64.
    const bytes = new Uint8Array(ciphertext)
    let binary = ''
    for (let i = 0; i < bytes.length; i++) {
      binary += String.fromCharCode(bytes[i])
    }
    return btoa(binary)
  } catch {
    return password
  }
}
