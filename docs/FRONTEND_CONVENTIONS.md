# Frontend Conventions

> Binding rules for `web/`. Must be kept aligned with `web/eslint.config.js` (the automated
> enforcement) and `docs/STACK.md` § Frontend Architecture (the layer/boundary contract these
> rules assume). This is convention, not aspiration — where ESLint can enforce a rule here, it
> does.

## 1. Files and components

- Source file/directory names: kebab-case. Component exports: PascalCase.
- One React component per file — a second component (even inline JSX assigned to a variable, or a
  nested component declaration) must be extracted to its own file/module.
- Components are arrow functions typed as `React.FC<Props>`.
- Use `type`, never `interface` (see § Types). Every slice **root** component keeps its props in a
  sibling `root-component.types.ts` file, inside a namespace named after the root component.
  Nested/child components only get this treatment when their types are non-trivial or shared
  across files.

```tsx
export namespace ExamplePageTypes {
  export type Props = { title: string };
}

export const ExamplePage: React.FC<ExamplePageTypes.Props> = (props) => {
  return <h1>{props.title}</h1>;
};
```

## 2. Access values without destructuring

- Access props via `props.name` — never destructure props in the function parameter list or body.
- Access object-valued hook results through a single named variable — do not destructure hook
  return values.
- The one exception: `useState` tuple destructuring (`const [value, setValue] = useState("")`).
- Rationale: keeps call sites searchable and makes the "owner" of each value visible at the use
  site.
- Scope: applies to components and custom hooks. Ordinary business-logic objects (non-hook,
  non-props) may still be destructured freely when it improves clarity.

```tsx
const routeData = Route.useLoaderData();
```

## 3. Effects

Every `useEffect` callback must be a **named function** whose name ends in `Fx`.

```tsx
useEffect(function persistStateFx() {
  stateStore.save();
}, [stateStore]);
```

## 4. Module structure and public APIs

Layer stack (FSD-like) — imports may only go within the same layer or strictly downward:

```
routes/    framework-owned route definitions, loaders and metadata
pages/     route-level composition
widgets/   reusable composite page chrome
features/  user-facing capabilities
entities/  reusable domain concepts
shared/    domain-agnostic config, helpers and styles
```

The exact enforced boundaries live in `web/eslint.config.js` and are documented authoritatively in
`docs/STACK.md` § Frontend Architecture — not duplicated here.

Per-slice internal file layout:

```
slice/
├── index.ts                       # the only cross-slice public API
├── root-component.tsx
├── root-component.types.ts        # RootComponentTypes namespace
├── root-component.module.css      # only when local styles exist
├── api/ | model/ | lib/           # retain only meaningful segments
└── components/                    # private composition
```

- Do NOT add an extra `ui/` segment. Existing `api/`, `model/`, `lib/` segments stay as-is — don't
  flatten them just for symmetry with slices that lack them.
- Cross-slice imports must target the slice directory and resolve through its `index.ts`; deep
  imports across slices are forbidden by ESLint. Within a slice, use relative imports.
- A private child component stays a single file under `components/` while simple; it earns its
  own recursive directory only once it owns types, styles, utilities, or child components of its
  own.
- Promote a component down to a more reusable/shared layer only after *real* cross-slice reuse has
  actually appeared — never build shared abstractions speculatively for hypothetical future reuse.
- Capability meaning is more important than matching the directory name to its root component —
  don't force a folder's name to mirror its root component's name if that obscures what the slice
  actually does.

## 5. Routing

- Use TanStack Router's typed route APIs directly: `Route.useLoaderData`, `Route.useParams`,
  `Route.useSearch`, `useNavigate`, `Link`.
- Do NOT introduce `useRouter` or `useTypedSearchParams` wrapper hooks — TanStack Router already
  derives precise types from the route tree, and a generic wrapper would hide route-specific types
  and reduce type safety rather than improve it.
- Route files stay thin and live only in the framework-fixed `src/routes/` location.
- Route component callbacks may keep the plain function shape TanStack Router expects and are
  exempt from the `React.FC` ESLint rule; however, the page components those route callbacks
  render are NOT exempt — they must still follow the `React.FC<Props>` rule from § 1.

## 6. Storage, JSON and environment

- Never touch `window.localStorage` directly outside `shared/lib/safe-ls` — use the versioned,
  SSR-safe `safeLs` API for all localStorage access.
- Use `shared/lib/safe-json` for persisted JSON (parsing/serializing data that will be stored).
  Plain `JSON.stringify` remains allowed specifically for an HTTP request body, since that's
  transport serialization, not storage.
- Only `shared/config/client-env.ts` and modules explicitly named `*.server.ts` are allowed to
  read environment variables directly; all other consumers must use those modules' typed exports.

## 7. Types

- Use `type` for object shapes, unions, and aliases — never `interface`. (Exception: generated
  files, e.g. `shared/api/schema.ts`, which use whatever the generator emits.)
- Literal TypeScript `namespace` declarations are allowed only inside `*.types.ts` files, and only
  to qualify ownership (e.g. `ExamplePageTypes.Props`); ESLint rejects namespaces everywhere else.
- Group module-level helper functions and constants into root-prefixed objects, e.g.
  `examplePageUtils`, `examplePageConstants`. Keep helpers pure. Do not create empty placeholder
  files for these.
- `*.dto.ts` is reserved strictly for transport/API-boundary shapes — never use it as a synonym
  for component props types.
- Export only the types that are actually consumed outside their own module (no speculative
  exports).
- Domain types belong to their owning entity — never duplicate an API response shape across
  multiple consumers.
- `src/routeTree.gen.ts` is generated code and is exempt from all authoring conventions.

## 8. Styling and interaction primitives

- Mantine and Recharts are prohibited dependencies. Use repository-owned semantic HTML/CSS and
  Base UI for headless interaction behavior that is genuinely non-trivial.
- `shared/config/design-tokens.ts` and `shared/styles/global.css` own global design values. Feature
  and widget CSS consumes semantic custom properties rather than introducing local palettes.
- Use CSS Modules for local static styles. Do not create `*.styles.ts` style-object files for
  static CSS; inline style is reserved for measured/dynamic values.
- Shared policy components (`ExternalLink`, `Image`, `Typography`, `PageContainer`) preserve
  accessibility and boundary rules. TanStack Router `Link` remains the internal-navigation
  primitive.
- `shared/ui` contains the small reusable control/layout vocabulary. It may adapt Base UI but may
  not recreate a general component framework or expose appearance-led props to domain components.
- uPlot is isolated behind the `widgets/live-chart` lifecycle owner. Canvas output always has a
  semantic textual or tabular alternative, and aggregation remains a backend responsibility.
- Keep tables, figures, code, details and lists native when their semantics are the point.

## 9. Testing

- New/changed pure logic, storage behavior, and API-client behavior gets a focused Vitest test
  under `web/tests/`, named `*.test.ts` or `*.test.tsx`. Use `web/tests/render.tsx` for the shared
  test-safe provider environment.
- New/changed browser journeys get a Playwright spec under `web/e2e/`.
- E2E specs must use Page Object Model classes from `web/e2e/pages/*.page.ts`, consumed only
  through typed fixtures (`web/e2e/fixtures.ts`) — specs never import Playwright primitives
  directly, never instantiate POMs themselves, and never touch Playwright locators/assertions/
  context/page APIs directly. Specs describe journeys; POMs own actions/assertions; fixtures own
  construction/teardown.
- Prefer user-visible locators (`getByRole`, `getByText`); do not add CSS selectors or
  `data-testid` solely to support tests.
- Test observable behavior, not implementation details. Stub network/global boundaries and restore
  them after each unit test; never make live network calls from unit tests.
- Unit and e2e tests run as part of `docs/STACK.md`'s Fast Gate / Full Gate — see that document
  for exactly which tier runs which command. There is no separate "local-only" carve-out here.
