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
// ============================================================================
// Group regions
//
// Administrators classify groups as serving overseas or domestic models
// (settings → group regions). A group serving overseas models requires the
// user's foreign-user declaration, so the UI marks those groups and prompts for
// the declaration before they can be picked.
// ============================================================================

export const GROUP_REGION_OVERSEAS = 'overseas'

export type GroupRegion = 'overseas' | 'domestic'

/** Whether a group region value marks a group serving overseas models. */
export function isOverseasRegion(region: string | undefined | null): boolean {
  return region === GROUP_REGION_OVERSEAS
}

/**
 * Read the group → region map published by `/api/status`. Unknown values are
 * dropped so a malformed payload cannot mark a domestic group as overseas.
 */
export function readGroupRegions(
  status: Record<string, unknown> | null | undefined
): Record<string, GroupRegion> {
  const raw = status?.group_regions
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {}
  const regions: Record<string, GroupRegion> = {}
  for (const [group, region] of Object.entries(
    raw as Record<string, unknown>
  )) {
    if (region === 'overseas' || region === 'domestic') {
      regions[group] = region
    }
  }
  return regions
}

/** Groups that serve overseas models, in a stable order. */
export function overseasGroupNames(
  regions: Record<string, GroupRegion>
): string[] {
  return Object.keys(regions)
    .filter((group) => isOverseasRegion(regions[group]))
    .sort((a, b) => a.localeCompare(b))
}
