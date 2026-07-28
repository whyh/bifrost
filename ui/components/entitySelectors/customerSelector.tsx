// Unified async customer selector — the single way to pick a governance
// customer (routing rules, model limits, pricing overrides).
//
// Lives in OSS because /governance/customers is an OSS endpoint; there is no
// enterprise customers API.
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
import { useGetCustomerQuery, useGetCustomersQuery } from "@/lib/store";
import { useMemo } from "react";

interface CustomerSelectorOwnProps extends EntitySelectorCommonProps {
	/** Page size for each search request. */
	limit?: number;
}

export type CustomerSelectorProps = CustomerSelectorOwnProps & EntitySelectorModeProps;

export function CustomerSelector({ limit = ENTITY_SELECTOR_PAGE_SIZE, ...props }: CustomerSelectorProps) {
	const { open, setOpen, setSearch, debouncedSearch, skip, isDebouncing } = useEntitySelectorSearch();

	const {
		data: customersData,
		isFetching,
		isError,
	} = useGetCustomersQuery({ limit, search: debouncedSearch || undefined }, { skip });

	// Memoized because multi mode hands this to react-select as defaultOptions,
	// which re-syncs on identity change.
	const options = useMemo(
		() =>
			(customersData?.customers ?? []).map((customer) => ({
				value: customer.id,
				label: customer.name || customer.id,
			})),
		[customersData],
	);

	// A customer picked before the popover ever opened has no label yet, so
	// fetch just that one rather than leaving its id on the trigger.
	const resolveId = unresolvedSelectionId(props);
	const { data: resolved } = useGetCustomerQuery(resolveId ?? "", { skip: !resolveId });
	const fallbackOption = useMemo(() => {
		const customer = resolved?.customer;
		if (customer && customer.id === resolveId) return { value: customer.id, label: customer.name || customer.id };
		return props.fallbackOption ?? null;
	}, [resolved, resolveId, props.fallbackOption]);

	return (
		<EntitySelector
			{...props}
			fallbackOption={fallbackOption}
			entityLabel="customer"
			entityLabelPlural="customers"
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
