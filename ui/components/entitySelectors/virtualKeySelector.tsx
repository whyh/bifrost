// Unified async virtual key selector — the single way to pick virtual keys
// across governance surfaces (model limits, pricing overrides, routing rules,
// MCP clients).
//
// Single mode is prop-compatible with the model limit scope picker contract
// ({ value, onChange, disabled, fallbackOption }), so it can be registered
// as-is in lib/registries/modelLimitScopes.tsx.

import {
	EntitySelector,
	ENTITY_SELECTOR_PAGE_SIZE,
	type EntitySelectorCommonProps,
	type EntitySelectorModeProps,
	type EntitySelectorOption,
	unresolvedSelectionId,
	useEntitySelectorSearch,
} from "@/components/entitySelectors/entitySelector";
import { useGetVirtualKeyQuery, useGetVirtualKeysQuery } from "@/lib/store";
import type { GetVirtualKeysParams } from "@/lib/types/governance";
import { useMemo } from "react";

export type VirtualKeySelectorOption = EntitySelectorOption;

/** Server-side filters beyond search/limit, e.g. scoping to a team or customer. */
export type VirtualKeySelectorFilters = Omit<GetVirtualKeysParams, "limit" | "offset" | "search" | "export">;

interface VirtualKeySelectorOwnProps extends EntitySelectorCommonProps {
	/** Page size for each search request. */
	limit?: number;
	/** Extra server-side filters merged into every request. */
	filters?: VirtualKeySelectorFilters;
}

export type VirtualKeySelectorProps = VirtualKeySelectorOwnProps & EntitySelectorModeProps;

export function VirtualKeySelector({ limit = ENTITY_SELECTOR_PAGE_SIZE, filters, ...props }: VirtualKeySelectorProps) {
	const { open, setOpen, setSearch, debouncedSearch, skip, isDebouncing } = useEntitySelectorSearch();

	const {
		data: vksData,
		isFetching,
		isError,
	} = useGetVirtualKeysQuery({ ...filters, limit, search: debouncedSearch || undefined }, { skip });

	// Memoized because multi mode hands this to react-select as defaultOptions,
	// which re-syncs on identity change. vk.value is the secret itself and is
	// deliberately never rendered.
	const options = useMemo(
		() =>
			(vksData?.virtual_keys ?? []).map((vk) => ({
				value: vk.id,
				label: vk.name || vk.id,
				description: vk.description,
			})),
		[vksData],
	);

	// A key picked before the popover ever opened has no label yet, so fetch
	// just that one rather than leaving its id on the trigger.
	const resolveId = unresolvedSelectionId(props);
	const { data: resolved } = useGetVirtualKeyQuery(resolveId ?? "", { skip: !resolveId });
	const fallbackOption = useMemo(() => {
		const vk = resolved?.virtual_key;
		if (vk && vk.id === resolveId) return { value: vk.id, label: vk.name || vk.id };
		return props.fallbackOption ?? null;
	}, [resolved, resolveId, props.fallbackOption]);

	return (
		<EntitySelector
			{...props}
			fallbackOption={fallbackOption}
			entityLabel="virtual key"
			entityLabelPlural="virtual keys"
			options={options}
			isFetching={isFetching}
			isError={isError}
			open={open}
			onOpenChange={setOpen}
			onSearchChange={setSearch}
			isSearching={isDebouncing || (isFetching && !!debouncedSearch)}
			debouncedSearch={debouncedSearch}
		/>
	);
}
