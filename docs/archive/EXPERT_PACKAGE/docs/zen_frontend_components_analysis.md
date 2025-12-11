---
⚠️ HISTORICAL DOCUMENT - EXPERT PACKAGE ARCHIVE ⚠️

This document is from an external "Expert Package" analysis of zen-watcher/ingester.
It reflects the state of zen-watcher at a specific point in time and may be partially obsolete.

CANONICAL SOURCES (use these for current direction):
- docs/PM_AI_ROADMAP.md - Current roadmap and priorities
- CONTRIBUTING.md - Current quality bar and standards
- docs/INFORMERS_CONVERGENCE_NOTES.md - Current informer architecture
- docs/STRESS_TEST_RESULTS.md - Current performance baselines

This archive document is provided for historical context, rationale, and inspiration only.
Do NOT use this as a replacement for current documentation.

---

# Zen Frontend Components and UI Patterns: A Strategic Blueprint for a Webhook Management Interface

## Executive Summary

The Zen React frontend exhibits a mature, modular architecture with clear boundaries between reusable UI primitives, data access, and feature surfaces. The codebase demonstrates:

- A coherent design system built on Tailwind CSS with a pragmatic “utility-first” approach, reinforced by MUI icons and a small set of shared UI components.
- State management anchored by React Query for server state, coupled with light-weight custom hooks and React Context for global concerns such as theme and infrastructure error state.
- Robust HTTP and WebSocket patterns, including an Axios instance with interceptors for correlation IDs, React Query-driven mutations and cache invalidation, and a WebSocket hook ready for HMAC authentication.
- A well-defined configuration-management UX: filterable lists, table-v2 patterns, wizard flows for setup, status pills, and toast notifications.
- A strong accessibility and responsiveness baseline: semantic HTML, focus-visible states, keyboard affordances, and responsive spacing and layout grids.

Primary gaps relate to backend endpoint maturity and authentication hardening for real-time features. The Integration Hub already treats webhooks as a first-class integration type. Building on this foundation, we recommend a dedicated Webhook Management interface that reuses the following proven components: FilterBarV2, DataTableV2, StatusPill, AppModal, SetupWizard, FormField/FormInput/FormTextarea/FormMultiSelect/FormSwitch/FormCheckbox, Toast, PageSkeletonV2, and ActionButton. These components provide a coherent and accessible configuration experience with test, validation, and activation workflows.

A high-value roadmap includes: a wizard for endpoint configuration and secrets, per-webhook test/dry-run, event-subscription management, inline secrets generation/rotation, batch actions, and WS-driven live status. This blueprint lays out the architecture, component reuse plan, end-to-end flows, accessibility and testing strategy, and a pragmatic implementation sequence.

## Methodology and Codebase Scope

This analysis focuses on the Zen React frontend repository and examines the directories and artifacts most relevant to UI composition, data fetching, configuration flows, and accessibility. The scope includes:

- React application and routing, with lazy-loaded routes for settings, policies, remediations v2, and v2 admin/ops surfaces.
- Component libraries under components/, with sub-categories for forms, modals, filters, data display, tables, and navigation.
- Hooks and contexts used for state, real-time updates, and cross-cutting concerns.
- API client configuration and test utilities for React Query and routing.

To anchor the observations, the following table inventories the primary codebase areas reviewed.

Table 1. Codebase scope inventory

| Directory | Key Contents | Notes |
|---|---|---|
| components/ | DataDisplay, FormComponents, ModalComponents, FilterComponents, common, tables, navigation, settings | A rich catalog of reusable primitives and composite components. |
| hooks/ | useApi, useWebSocket, useIntegrations, useRemediations, useTenantId, useUiAnalytics, useLiveQuery | Cohesive hook-based state management for server state and UI concerns. |
| api/ | http, axios-instance.ts, generated client types | Axios interceptors, same-origin proxying, cancel tokens. |
| contexts/ | ThemeContext, TenantContext, InfraErrorContext | Global state for theme, tenant scoping, and infra errors. |
| pages/ | Settings, Policies, Exceptions, Schedules, Admin/Ops v2, Notifications | Feature surfaces for configuration and operations. |
| tests/ | Unit, e2e (Playwright), a11y, performance | Evidence of accessibility gates and smoke tests. |

Key observations across the codebase include route-level code-splitting, feature flags gating nav v2 and remediations v2, consistent loading skeletons, and robust error handling in integration-centric flows.

## Component Library and Design System

Zen’s design system leans into Tailwind CSS for styling, complemented by MUI icons and a small, well-scoped set of UI primitives. This approach emphasizes consistency with utility classes while allowing shared components to encode interaction patterns (validation, loading, disabled states) that reduce duplication across features.

Design tokens and theming are implemented through a ThemeContext that supports light, dark, and system preferences. The context resolves system preference, persists user choice, and applies a dark class to the document root. This enables component-level dark mode support with minimal boilerplate.

The component catalog is deep. It includes a FormComponents suite for inputs, selects, checkboxes, radios, sliders, date pickers, multi-select, file upload, and a wizard. ModalComponents provide dialogs, alert/confirm patterns, form modals, popovers, and a Toast. FilterComponents provide a filter bar, chips, panels, saved filters, and multi-select/slider filters. DataDisplay includes badges, status indicators, charts, data tables, virtualized tables, and kanban boards. Tables includes bulk actions and enhanced data table patterns. Navigation components include a v2 sidebar and breadcrumbs. The UI layer includes ActionButton and HealthCard.

These building blocks combine to form composite patterns: list-then-detail with filters, wizard-driven setup flows, modals for preview and confirmation, toasts for feedback, and skeletons for progressive loading.

Table 2 catalogs representative reusable components and their intended use.

Table 2. Reusable component catalog

| Component | Location | Purpose | Typical Use Case | Dependencies |
|---|---|---|---|---|
| FormField, FormInput, FormTextarea, FormSelect, FormMultiSelect, FormCheckbox, FormRadio, FormSlider, FormDatePicker, FormUpload, FormWizard | components/FormComponents | Accessible form controls with validation and loading states | Configuration forms, setup wizards, settings pages | React Hook Form, Zod resolvers, Tailwind |
| AppModal, AlertDialog, ConfirmDialog, FormModal, Popover, Toast | components/ModalComponents | Dialogs, confirmations, form modals, inline notifications | Edit/Create flows, confirmation prompts, preview panels | Tailwind, Headless interaction logic |
| FilterBarV2, FilterChip, FilterPanel, SavedFilters, MultiSelectFilter, SliderFilter, SearchFilter | components/FilterComponents | Filter/sort/search with URL synchronization | List pages (Integrations, Remediations v2) | React Router search params |
| DataTableV2, VirtualizedTable, EnhancedDataTable, BulkActionsBar | components/DataDisplay, components/tables | Sortable/filterable tables with bulk actions | Integrations list, audit logs, reports | TanStack Table v8 |
| StatusPill, Badge, TagsInput, Timeline, StatsCard, Chart | components/DataDisplay | Status and metadata visualization | Health/status badges, trend charts | Tailwind; Recharts for charts |
| NavigationV2, Breadcrumbs, TopBar | components/navigation | Layout chrome for v2 surfaces | Admin/Ops v2 pages | React Router |
| PageSkeletonV2, SkeletonV2 | components/common | Progressive loading states | Any list or detail page | Tailwind |
| ActionButton | components/ui | Action hierarchy with focus styles | Any trigger: save, test, apply | Tailwind |

Table 3 summarizes design primitives and their roles.

Table 3. Design primitives inventory

| Primitive | Description | Notes |
|---|---|---|
| Tailwind utility classes | Utility-first styling for spacing, color, typography | Applied consistently; dark mode via class on document root |
| Icons (MUI, Lucide) | Iconography for actions, status, and navigation | Used across tables, buttons, and navigation |
| StatusPill/Badge variants | Standardized semantic color tokens | Badges for severity and state; mapped consistently in lists |
| Focus and disabled states | Focus ring, opacity, cursor not-allowed | Encoded in ActionButton and form components |

### Tailwind and ThemeContext Integration

Tailwind is configured with a standard content globs and an extended theme, with safelisted badge and state classes to ensure generated classnames are included in the build. The ThemeContext resolves light/dark based on user preference or system default and toggles a dark class on the document root. This arrangement ensures that all components—including badges, modals, and inputs—render correctly across themes without ad-hoc styling.

### Forms and Validation Patterns

The form library is comprehensive and consistent. Form components surface validation, helper text, error messages, loading indicators, and flows. While the codebase references React Hook Form and Zod, the explicit resolver wiring is not enumerated in the context and should be verified during webhook form implementation.

Table 4. Form components matrix

| Component | Key Props | Validation Approach | Accessibility Features |
| FormField ||---|---|---|---|
 disabled, loading, helper, support multi-step setup label, required, disabled states. Wizards error | Wraps control; displays error/success/warning | ARIA attributes on field and message association |
| FormInput | type, placeholder, icon, validation, showPasswordToggle | Inline validation with callbacks | Label association; invalid/required states |
| FormSelect | options, searchable, clearable, multiple | Client-side and async validation hooks | Keyboard navigation; aria-expanded |
| FormMultiSelect | options, maxSelected, showTags | Selection limits; tag-based UI | Tag roles; keyboard interaction |
| FormCheckbox | variant (default/switch/toggle), size | Boolean validation | Large target areas; screen-reader labels |
| FormRadio | options, layout, variant | Single selection validation | Group roles; focus management |
| FormTextarea | autoResize, min/max rows, maxLength | Length validation | Character count; label association |
| FormDatePicker | showTime, min/max dates, disable weekends | Date/time constraints | Keyboard navigation; locale formatting |
| FormSlider | min/max, step, marks, showValue | Range validation | ARIA valuetext; marks as labels |
| FormUpload | multiple, accept, maxSize, showPreview | File type/size validation | Drag/drop semantics; progress updates |
| FormWizard | steps, onComplete, linearProgression | Per-step validation hooks | Stepper semantics; progress announcements |

### Modals, Toasts, and Feedback Patterns

Modal components provide a standard dialog surface with header, body, and footer slots. AlertDialog and ConfirmDialog encapsulate destructive actions and require explicit user confirmation. FormModal binds a form within a modal workflow, reducing page transitions. Toast offers lightweight feedback for success, info, and error states, used extensively after mutation operations. EmptyState and PageSkeletonV2 complement feedback by clarifying non-loading and loading conditions.

### Data Display and Table Patterns

The DataTableV2 pattern supports sorting, filtering, row actions, and empty/error states. For large datasets, VirtualizedTable improves performance. EnhancedDataTable and BulkActionsBar enable multi-select and batch operations. These patterns are leveraged in integration lists and should be reused for webhook lists and audit logs.

Table 5. Table feature coverage

| Feature | Component(s) | Example Usage |
|---|---|---|
| Sorting | DataTableV2 | IntegrationHub table with sortable name/status/type/last test |
| Filtering | FilterBarV2 + DataTableV2 | URL-synchronized filters and sort |
| Pagination | DataTableV2 (assumed) | List pages with page/pageSize |
| Row Actions | DataTableV2 | Configure/Test actions per row |
| Bulk Actions | BulkActionsBar, EnhancedDataTable | Bulk enable/disable, rotate secrets |
| Virtual Scrolling | VirtualizedTable | Large webhook event logs |
| Empty/Error States | DataTableV2 + EmptyState | Informative empty states with CTAs |

### Navigation and Layout

NavigationV2 provides a v2 sidebar, TopBar includes global search and help, and Breadcrumbs support wayfinding. The layout is responsive with content areas sized using utility classes. A global onboarding wizard and a NotificationBell complement the layout chrome for user guidance and awareness.

## State Management Patterns

Server state is handled via TanStack React Query (QueryClient), with queries and mutations in feature hooks. Client state is kept local where possible and promoted to context when it must be shared globally or cross-route (theme, infra errors, tenant scoping). The approach yields clean separation of concerns and testability.

Table 6. State ownership map

| State | Owner/Hook | Scope | Notes |
|---|---|---|---|
| Integrations list | useIntegrations | Per-tenant | React Query cache; invalidation on save/test |
| Test mutation | useTestIntegration | Per-tenant | Pending states; toast feedback |
| Save config | useSaveIntegrationConfig | Per-tenant | Invalidate queries on success |
| Live WS updates | useWebSocket | Global | Subscriptions for updates |
| Theme | ThemeContext | Global | Persisted to localStorage |
| Infra errors | InfraErrorContext | Global | Displays outage banners |
| Tenant | TenantContext | Global | TenantId required for queries |
| UI analytics | useUiAnalytics | Global | Event tracking |
| Page title | usePageTitle | Local | Document title updates |

### React Query and Custom Hooks

Custom hooks encapsulate queries/mutations and provide typed data and error surfaces. The IntegrationHub demonstrates the pattern: useIntegrations fetches, useTestIntegration tests connectivity, and useSaveIntegrationConfig persists configuration. QueryClient invalidation keeps lists fresh after mutations, and Toast notifications deliver immediate feedback. Test utilities provide a QueryClient wrapper to standardize testing for hooks and components.

### Contexts and Cross-Cutting Concerns

ThemeContext resolves the effective theme and applies it to the DOM. InfraErrorContext propagates infrastructure outage information to a banner surface in the layout, informing users during degraded conditions. TenantContext provides tenant scoping for queries; several flows fall back to a placeholder tenant when none is available, which should be removed in production to prevent accidental cross-tenant data exposure.

### Real-time and Live State

A WebSocket hook supports connection, subscription, and message handling. It currently subscribes to cluster health and remediation updates. The hook includes placeholders for HMAC-based query parameters (signature, timestamp, nonce, user and tenant IDs). Production hardening should implement signing and validation to prevent spoofing. When connected, live updates can be integrated into React Query via queryClient.setQueryData or invalidation strategies, ensuring the UI remains current without manual refresh.

Table 7. WebSocket topics and UI consumers

| Topic | Consumer | Update Strategy |
|---|---|---|
| cluster_health_updates | Health dashboards | Invalidate or patch health queries |
| remediation_updates | Remediations v2 | Update list/detail caches; show toasts |

## API Integration and Data Fetching

HTTP calls are made through a centralized Axios instance configured for a same-origin API proxy, with credentials enabled. Interceptors inject X-Request-Id on mutating requests to support idempotency and traceability. A helper provides cancel tokens to abort in-flight requests when needed.

React Query drives data fetching, caching, retries, and invalidation. Mutations like test integration or save configuration follow a consistent lifecycle: pending state, success toast, and cache invalidation.

Codegen tooling (Orval and openapi-zod-client) generates typed hooks and models from OpenAPI specifications. A post-processing script aligns generated outputs with the codebase conventions. The HTTP and codegen patterns together yield strong type safety and API consistency.

Error handling is visible at both the component level (error states, retry calls) and the BFF layer (central error handler). Toasts communicate outcomes, and empty states provide actionable guidance.

Table 8. API layer summary

| Layer | Responsibilities | Notable Details |
|---|---|---|
| HTTP client (Axios) | Base URL, interceptors, cancel tokens | X-Request-Id on mutating ops; credentials enabled |
| React Query | Queries, mutations, cache, retries | Invalidation on save/test; test utilities for hooks |
| Generated client | Typed hooks/models from OpenAPI | Orval + openapi-zod-client; post-processing script |
| Error handling | Component and BFF-level | Toasts, empty states, infra outage banners |

Table 9. Codegen artifacts and consumers

| Artifact | Source Spec | Consumer Components |
|---|---|---|
| Generated hooks/models | OpenAPI (BFF/back) | Feature hooks; pages; tables |
| Post-processed outputs | Script alignment | IntegrationHub, remediations v2, admin/ops pages |

## User Interface for Configuration Management

Configuration UIs follow a consistent pattern: a FilterBarV2 above a DataTableV2 with row-level actions and modals for edit/create flows. Setup wizards guide users through multi-step configuration. Status and metadata are surfaced via StatusPill and iconography. URL search params maintain filter/sort state, enabling deep-linking and back/forward navigation.

Table 10. Configuration UX patterns

| Pattern | Components | Example |
|---|---|---|
| Filter → Table → Actions | FilterBarV2, DataTableV2, ActionButton | IntegrationHub list with configure/test |
| Wizard for setup | FormWizard, FormField suite | GitHub/Jira setup wizards |
| Modal for edit/preview | AppModal, FormModal | Editing webhook configuration |
| Feedback and states | Toast, EmptyState, PageSkeletonV2 | Post-mutation toasts; empty lists |

### Integration Hub: Configuration and Test Flows

The IntegrationHub lists integrations by type and status, provides search and filter controls synchronized to URL params, and supports sorting by name, status, type, or last test. Actions include Configure/Edit and Test. Successful configuration or test triggers React Query invalidation and toasts. The hub includes sensible error handling and retry affordances, and uses PageSkeletonV2 during loading.

Table 11. IntegrationHub columns and actions

| Column | Description | Actions |
|---|---|---|
| Name (with icon) | Integration name and optional repository | Configure/Edit |
| Type | Integration type (e.g., webhook) | — |
| Status | Connected/Not configured/Partial/Error | Test (if connected) |
| Last Test | Timestamp or “Never” | — |
| Metadata | Org/Project/Workspace details | — |
| Actions | Buttons for Configure/Edit and Test | — |

## Real-time Updates and Notifications

WebSocket connectivity is encapsulated in a dedicated hook that manages connection lifecycle, topic subscriptions, and message parsing. It is prepared for HMAC authentication via query parameters, though production requires full signing and validation. Notifications appear via a Toast system and a NotificationBell; the latter is integrated into the top bar.

Table 12. Real-time touchpoints

| Topic | UI Consumer | Update Strategy |
|---|---|---|
| cluster_health_updates | Cluster dashboards | Invalidate or patch queries |
| remediation_updates | Remediations v2 | Invalidate list/detail and toast |
| General notifications | NotificationBell | Pull-based or WS-pushed counts |

## Responsive Design and Accessibility

Responsive design is achieved through Tailwind utilities for spacing, grid, and visibility. The layout supports fluid content areas and consistent padding/margins. Accessibility is addressed via semantic HTML, ARIA attributes in form components, focus-visible styles, keyboard shortcuts (e.g., global search), and project-level Playwright a11y gates.

Table 13. Accessibility evidence map

| Feature | Components | WCAG Outcome |
|---|---|---|
| Semantic roles and labels | Form components, dialogs | Name, role, value conveyed |
| Keyboard navigation | Global search/help, modals | Operable via keyboard |
| Focus styles | ActionButton, inputs | Visible focus indicators |
| ARIA states | Invalid/required, expanded | Status communicated to AT |
| A11y test gates | Playwright a11y project | Automated checks in CI |

## Webhook Management Interface: Reusable Components and End-to-End Design

Webhooks are already represented as a first-class integration type in the IntegrationHub, complete with status handling and actions. Building on this, we recommend a dedicated Webhook Management surface that reuses established components and flows:

- List and filter webhooks with FilterBarV2 and DataTableV2, including URL-synchronized filters and sort.
- Add/Edit flows via a SetupWizard that collects URL, secret, and event subscriptions, with inline validation and helper text.
- Testing via per-webhook “Test” actions, with toast feedback and optional response preview in a modal.
- Activation state management through a StatusPill and row-level actions.
- Empty states with guidance for first-time setup and post-failure recovery.

Table 14. Webhook UI components mapping

| Feature Step | Existing Components | Adaptation Notes |
|---|---|---|
| List/filter | FilterBarV2, DataTableV2, StatusPill | Sync filters to URL; add activation column |
| Create/Edit | FormWizard, FormField, FormInput, FormTextarea, FormMultiSelect, FormCheckbox | Validate URL/secret; event subscriptions |
| Test | ActionButton, Toast, AppModal (optional preview) | Show success/failure toasts; capture response metadata |
| Secrets | FormInput (masked), FormSwitch (show/hide) | Encourage secret rotation flows |
| Loading/empty | PageSkeletonV2, EmptyState | Guide first-time configuration |
| Batch ops | BulkActionsBar, EnhancedDataTable | Bulk enable/disable, secret rotation |

### Proposed Screen: Webhook List

The list view displays Name, Status (StatusPill), Last Test, Event Types, and Actions. Filters include status, event type, and search. Sorting includes last test recency. Empty states suggest adding a webhook or adjusting filters. Bulk actions support enable/disable and secret rotation.

Table 15. Webhook list columns

| Column | Description | Sort | Filter |
|---|---|---|---|
| Name | Webhook name/identifier | Yes | Search |
| Status | Activation state (healthy/warning/critical/offline) | Yes | Status |
| Last Test | Timestamp or “Never” | Yes | — |
| Event Types | Subscribed events (multi-select badge) | — | Event type |
| Actions | Test, Edit, Disable/Enable | — | — |

### Proposed Screen: Webhook Create/Edit Wizard

Step 1 captures the endpoint URL and optional secret, with inline validation. Step 2 selects event types via a multi-select control. Step 3 reviews and tests the configuration, showing a dry-run result. Step 4 saves and activates.

Table 16. Wizard steps and controls

| Step | Fields | Validation | Outcome |
|---|---|---|---|
| 1. Endpoint | URL (required), Secret (optional, masked) | URL format; secret length | Ready to test |
| 2. Events | Event types (multi-select) | At least one event | Subscriptions set |
| 3. Review/Test | Summary + Test button | — | Dry-run result |
| 4. Save/Activate | Confirm | — | Webhook saved; status active |

## Risks, Gaps, and Mitigation Plan

Several constraints and gaps require attention:

- Backend readiness: some integration endpoints and webhook-specific APIs are pending; the UI includes placeholders and informative toasts.
- WebSocket HMAC authentication is planned but not implemented; production requires signing and validation to prevent abuse.
- Tenant handling includes placeholders for test-only scenarios; production must ensure robust tenant scoping.
- Incomplete design tokens beyond Tailwind utilities; elevation, motion, and dark-mode semantics could be formalized.
- Limited React Hook Form + Zod resolver evidence in the context; verify wiring during webhook form build.
- Real-time event catalog is implicit; define a webhook delivery event taxonomy for UI consumption.
- Feature flag sprawl risk; continue consolidating flags and ensure guardrails for P0 routes.

Table 17. Risk register

| Risk | Impact | Evidence | Mitigation |
|---|---|---|---|
| Backend endpoints pending | UI cannot save/test webhooks | IntegrationHub modal messaging | Stub BFF endpoints; stage rollout; feature flag |
| WS HMAC auth missing | Security risk for real-time | WS hook comments | Implement signing/validation; rotate secrets |
| Placeholder tenant IDs | Cross-tenant data exposure risk | Tenant hook fallback | Enforce tenant context; remove placeholders |
| Limited design tokens | Inconsistent elevation/motion | Tailwind-only theming | Introduce token map; codify dark-mode semantics |
| Form+resolver wiring unclear | Validation inconsistencies | Form library docs | Verify RHF+Zod wiring in webhook forms |
| Event catalog unspecified | Unclear UI for events | WS topics only | Define event taxonomy and mapping |
| Feature flag sprawl | Complexity and regressions | NavV2/RemediationsV2 flags | Centralize flag governance; audit gates |

## Implementation Roadmap and Milestones

A phased plan reduces risk and aligns with backend readiness.

Table 18. Roadmap

| Phase | Scope | Components | Acceptance Criteria | Dependencies |
|---|---|---|---|---|
| 1. List & Filters | Webhook list with FilterBarV2/DataTableV2 | FilterBarV2, DataTableV2, StatusPill | Filter/sort in URL; empty/loading states | BFF list endpoint |
| 2. Wizard | Create/Edit webhook via wizard | FormWizard, FormField suite | End-to-end create/edit; validation | Config save endpoint |
| 3. Test & Preview | Per-webhook test + response preview | ActionButton, Toast, AppModal | Toasts on success/failure; optional preview | Test endpoint |
| 4. Secrets Mgmt | Secret create/rotate flows | FormInput masked, FormSwitch | Rotation updates status; audit log | Secrets endpoint |
| 5. Batch Ops | Enable/disable, rotate secrets | BulkActionsBar, EnhancedDataTable | Batch success/failure handling | Bulk endpoints |
| 6. Real-time | WS-driven status and events | useWebSocket, Toast | Live status pills; event feed | HMAC auth; event taxonomy |
| 7. Hardening | A11y, perf, i18n | Playwright a11y, VirtualizedTable | WCAG gates; smooth scrolling; localized strings | Test harness; i18n strings |

## Appendix: Evidence Map and Source Inventory

Table 19. Evidence map

| Area | Files/Sections | Purpose | Notes |
|---|---|---|---|
| Routing & layout | App routes and layout components | Route guard, lazy-loading, feature flags | P0 route hygiene; nav v2 flag |
| Forms | Form component library and docs | Validation, accessibility | Wizard-driven flows |
| Modals & feedback | ModalComponents, Toast | Dialogs, confirmations, toasts | Reused across features |
| Filters & tables | FilterBarV2, DataTableV2 | List UX pattern | URL synchronization |
| Integration Hub | IntegrationHub component | Configuration and test flows | Webhook as integration type |
| Types | Integrations types | WebhookConfig shape | URL, secret, events |
| HTTP client | Axios interceptors | X-Request-Id, cancel tokens | Same-origin proxy |
| React Query | Test utilities | QueryClient wrapper | Consistent hook tests |
| WS | WebSocket hook | Real-time subscriptions | HMAC placeholders |
| Contexts | Theme/Infra/Tenant | Global state | Dark mode; outage banners |

Information gaps to acknowledge:

- Specific backend endpoints for webhooks (create/update/delete/test) are not fully enumerated.
- Webhook event catalog and payload schema for real-time updates are not defined in the context.
- Secret management UX (generation, rotation, storage) requires backend contracts and security review.
- Granular RBAC and permissions for webhook operations are not detailed.
- Feature flag governance needs consolidation; current flags are centralized but scope is evolving.
- Design token catalog beyond Tailwind (elevation, motion, dark-mode semantics) is limited.
- React Hook Form + Zod resolver usage is referenced by components but explicit examples are not included.

---

By leveraging Zen’s existing component library and patterns, the proposed Webhook Management interface can be delivered quickly, safely, and consistently—accelerating time-to-value while preserving accessibility, maintainability, and security.