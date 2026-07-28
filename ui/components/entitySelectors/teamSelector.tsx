// Unified async team selector — the single way to pick a governance team
// (routing rules, model limits, pricing overrides).
//
// Lives in OSS because the governance teams endpoint (/governance/teams) is
// OSS. Note this is a different entity from the enterprise user-groups team
// list (/teams), which carries membership and business-unit fields and is
// only used by the admin surfaces under Users & Groups.
//
// Single mode is prop-compatible with the model limit scope picker contract
// ({ value, onChange, disabled, fallbackOption }).

import {
	EntitySelector,
	ENTITY_SELECTOR_PAGE_SIZE,
	type EntitySelectorCommonProps,
	type EntitySelectorModeProps,
	unresolvedSelectionId,
	useEntitySelectorSearch,
} from "@/components/entitySelectors/entitySelector";
import { useGetTeamQuery, useGetTeamsQuery } from "@/lib/store";
import type { GetTeamsParams } from "@/lib/types/governance";
import { useMemo } from "react";

/** Server-side filters beyond search/limit, e.g. scoping to one customer. */
export type TeamSelectorFilters = Omit<GetTeamsParams, "limit" | "offset" | "search">;

interface TeamSelectorOwnProps extends EntitySelectorCommonProps {
	/** Page size for each search request. */
	limit?: number;
	/** Extra server-side filters merged into every request. */
	filters?: TeamSelectorFilters;
}

export type TeamSelectorProps = TeamSelectorOwnProps & EntitySelectorModeProps;

export function TeamSelector({ limit = ENTITY_SELECTOR_PAGE_SIZE, filters, ...props }: TeamSelectorProps) {
	const { open, setOpen, setSearch, debouncedSearch, skip, isDebouncing } = useEntitySelectorSearch();

	const {
		data: teamsData,
		isFetching,
		isError,
	} = useGetTeamsQuery({ ...filters, limit, search: debouncedSearch || undefined }, { skip });

	// Memoized because multi mode hands this to react-select as defaultOptions,
	// which re-syncs on identity change.
	const options = useMemo(
		() =>
			(teamsData?.teams ?? []).map((team) => ({
				value: team.id,
				label: team.name || team.id,
				// The customer a team rolls up to is the only thing that
				// disambiguates two teams sharing a name.
				description: team.customer?.name,
			})),
		[teamsData],
	);

	// A team picked before the popover ever opened has no label yet, so fetch
	// just that one rather than leaving its id on the trigger.
	const resolveId = unresolvedSelectionId(props);
	const { data: resolved } = useGetTeamQuery(resolveId ?? "", { skip: !resolveId });
	const fallbackOption = useMemo(() => {
		const team = resolved?.team;
		if (team && team.id === resolveId) return { value: team.id, label: team.name || team.id };
		return props.fallbackOption ?? null;
	}, [resolved, resolveId, props.fallbackOption]);

	return (
		<EntitySelector
			{...props}
			fallbackOption={fallbackOption}
			entityLabel="team"
			entityLabelPlural="teams"
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
