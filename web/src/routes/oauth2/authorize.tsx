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
import { createFileRoute } from '@tanstack/react-router'

/**
 * OAuth 2.0 consent is server-rendered at /oauth2/authorize (see
 * router/oauth-router.go). The SPA only ever sees this URL through a
 * client-side navigation from the sign-in page after a successful login, at
 * which point the session cookie exists — so force a full page load and let
 * the backend render the consent screen. A direct browser visit never reaches
 * this route: the server handles it first.
 */
export const Route = createFileRoute('/oauth2/authorize')({
  beforeLoad: () => {
    window.location.assign(window.location.href)
  },
})
