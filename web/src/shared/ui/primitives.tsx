import { Button as BaseButton } from "@base-ui/react/button";
import type React from "react";
import styles from "./primitives.module.css";

type LayoutProps = React.HTMLAttributes<HTMLElement> & {
  children?: React.ReactNode;
  gap?: string | number;
  justify?: "center" | "flex-end" | "flex-start" | "space-between";
  align?: "center" | "flex-end" | "flex-start" | "stretch";
  wrap?: "nowrap" | "wrap" | "wrap-reverse";
  mb?: string | number;
  p?: string | number;
  py?: string | number;
  w?: string | number;
  mih?: string | number;
  flex?: string | number;
  component?: "div" | "nav";
};

const space = (value?: string | number) =>
  typeof value === "number"
    ? value
    : value
      ? `var(--space-${value}, ${value})`
      : undefined;

export const Stack: React.FC<LayoutProps> = (props) => {
  const Tag = props.component ?? "div";
  return (
    <Tag
      className={`${styles.stack} ${props.className ?? ""}`}
      style={{
        gap: space(props.gap),
        justifyContent: props.justify,
        alignItems: props.align,
        marginBottom: space(props.mb),
        padding: space(props.p),
        paddingBlock: space(props.py),
        width: props.w,
        minHeight: props.mih,
        ...props.style,
      }}
    >
      {props.children}
    </Tag>
  );
};
export const Group: React.FC<LayoutProps> = (props) => (
  <div
    className={`${styles.group} ${props.className ?? ""}`}
    style={{
      gap: space(props.gap),
      justifyContent: props.justify,
      alignItems: props.align,
      flexWrap: props.wrap,
      marginBottom: space(props.mb),
      padding: space(props.p),
      minHeight: props.mih,
      flex: props.flex,
      ...props.style,
    }}
  >
    {props.children}
  </div>
);
export const Box: React.FC<LayoutProps> = (props) => (
  <div
    style={{
      marginBottom: space(props.mb),
      padding: space(props.p),
      minHeight: props.mih,
      flex: props.flex,
      ...props.style,
    }}
  >
    {props.children}
  </div>
);
export const Center: React.FC<LayoutProps> = (props) => (
  <div
    className={styles.center}
    style={{ minHeight: props.mih, ...props.style }}
  >
    {props.children}
  </div>
);
export const SimpleGrid: React.FC<
  LayoutProps & { cols?: unknown; spacing?: string | number }
> = (props) => (
  <div className={styles.grid} style={{ gap: space(props.spacing) }}>
    {props.children}
  </div>
);
export const Card: React.FC<
  LayoutProps & {
    withBorder?: boolean;
    padding?: string;
    radius?: string;
    bg?: string;
  }
> = (props) => (
  <section
    className={`${styles.card} ${props.className ?? ""}`}
    style={props.style}
  >
    {props.children}
  </section>
);

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  loading?: boolean;
  variant?: string;
  color?: string;
  fullWidth?: boolean;
};
function buttonDomProps(
  props: ButtonProps,
): React.ButtonHTMLAttributes<HTMLButtonElement> {
  const result = { ...props };
  delete result.loading;
  delete result.variant;
  delete result.color;
  delete result.fullWidth;
  return result;
}
export const Button: React.FC<ButtonProps> = (props) => (
  <BaseButton
    {...buttonDomProps(props)}
    className={`${styles.button} ${props.variant === "light" ? styles.secondary : ""} ${props.className ?? ""}`}
    style={{ width: props.fullWidth ? "100%" : undefined, ...props.style }}
    disabled={props.disabled || props.loading}
  >
    {props.loading ? "Working…" : props.children}
  </BaseButton>
);
export const ActionIcon: React.FC<ButtonProps> = (props) => (
  <BaseButton
    {...buttonDomProps(props)}
    className={`${styles.iconButton} ${props.className ?? ""}`}
  >
    {props.children}
  </BaseButton>
);

type InputProps = Omit<
  React.InputHTMLAttributes<HTMLInputElement>,
  "onChange"
> & {
  label: string;
  description?: string;
  onChange?: React.ChangeEventHandler<HTMLInputElement>;
};
function inputDomProps(
  props: InputProps & { inputType?: string },
): React.InputHTMLAttributes<HTMLInputElement> {
  const result: Partial<InputProps & { inputType?: string }> = { ...props };
  delete result.label;
  delete result.description;
  delete result.inputType;
  return result as React.InputHTMLAttributes<HTMLInputElement>;
}
const InputField: React.FC<InputProps & { inputType?: string }> = (props) => (
  <label className={styles.field}>
    <span>{props.label}</span>
    <input
      {...inputDomProps(props)}
      type={props.inputType ?? props.type}
      className={styles.input}
    />
    <small>{props.description}</small>
  </label>
);
export const TextInput: React.FC<InputProps> = (props) => (
  <InputField {...props} />
);
export const PasswordInput: React.FC<InputProps> = (props) => (
  <InputField {...props} inputType="password" />
);
type NumberProps = Omit<InputProps, "value" | "onChange"> & {
  value?: number | string;
  min?: number;
  max?: number;
  onChange?: (value: number | string) => void;
};
export const NumberInput: React.FC<NumberProps> = (props) => (
  <InputField
    {...props}
    inputType="number"
    onChange={(event) =>
      props.onChange?.(
        event.currentTarget.value === ""
          ? ""
          : Number(event.currentTarget.value),
      )
    }
  />
);
type SelectProps = {
  label: string;
  description?: string;
  placeholder?: string;
  required?: boolean;
  clearable?: boolean;
  data: Array<string | { value: string; label: string }>;
  value: string | null;
  onChange: (value: string | null) => void;
};
export const Select: React.FC<SelectProps> = (props) => (
  <label className={styles.field}>
    <span>{props.label}</span>
    <select
      className={styles.input}
      required={props.required}
      value={props.value ?? ""}
      onChange={(event) => props.onChange(event.currentTarget.value || null)}
    >
      <option value="">
        {props.placeholder ?? (props.clearable ? "None" : "Choose")}
      </option>
      {props.data.map((item) => {
        const value = typeof item === "string" ? item : item.value;
        const label = typeof item === "string" ? item : item.label;
        return (
          <option key={value} value={value}>
            {label}
          </option>
        );
      })}
    </select>
    <small>{props.description}</small>
  </label>
);
type CheckProps = Omit<React.InputHTMLAttributes<HTMLInputElement>, "type"> & {
  label?: string;
  description?: string;
};
export const Checkbox: React.FC<CheckProps> = (props) => (
  <label className={styles.check}>
    <input
      {...inputDomProps({ ...props, label: props.label ?? "" })}
      type="checkbox"
    />
    <span>{props.label}</span>
    <small>{props.description}</small>
  </label>
);
export const Switch: React.FC<CheckProps> = (props) => (
  <input
    {...inputDomProps({ ...props, label: props.label ?? "" })}
    type="checkbox"
    role="switch"
    className={styles.switch}
  />
);
export const Alert: React.FC<LayoutProps & { color?: string }> = (props) => (
  <div role="status" className={styles.alert}>
    {props.children}
  </div>
);
export const Loader: React.FC = () => (
  <span className={styles.loader} aria-label="Loading" />
);

type DrawerProps = {
  opened: boolean;
  onClose: () => void;
  title: string;
  position?: string;
  children: React.ReactNode;
};
export const Drawer: React.FC<DrawerProps> = (props) =>
  props.opened ? (
    <div
      className={styles.overlay}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) props.onClose();
      }}
    >
      <aside className={styles.drawer} aria-label={props.title}>
        <Group justify="space-between">
          <h2>{props.title}</h2>
          <ActionIcon aria-label="Close" onClick={props.onClose}>
            ×
          </ActionIcon>
        </Group>
        {props.children}
      </aside>
    </div>
  ) : null;

const TableRoot: React.FC<React.TableHTMLAttributes<HTMLTableElement>> = (
  props,
) => (
  <div className={styles.tableWrap}>
    <table {...props} className={styles.table}>
      {props.children}
    </table>
  </div>
);
export const Table = Object.assign(TableRoot, {
  Thead: (props: React.HTMLAttributes<HTMLTableSectionElement>) => (
    <thead {...props} />
  ),
  Tbody: (props: React.HTMLAttributes<HTMLTableSectionElement>) => (
    <tbody {...props} />
  ),
  Tr: (props: React.HTMLAttributes<HTMLTableRowElement>) => <tr {...props} />,
  Th: (props: React.ThHTMLAttributes<HTMLTableCellElement>) => (
    <th {...props} />
  ),
  Td: (props: React.TdHTMLAttributes<HTMLTableCellElement>) => (
    <td {...props} />
  ),
});
