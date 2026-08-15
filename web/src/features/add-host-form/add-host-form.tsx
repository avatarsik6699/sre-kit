import { useState } from "react";
import {
  Alert,
  Button,
  Group,
  NumberInput,
  Stack,
  Textarea,
  TextInput,
} from "@mantine/core";
import type { AddHostFormTypes } from "./add-host-form.types";

const INITIAL_VALUES: AddHostFormTypes.Values = {
  label: "",
  address: "",
  sshPort: 22,
  sshUser: "",
  sshKey: "",
};

/**
 * Host setup form (docs/SPEC.md §12.1). Pastes an existing private key rather than sre-kit
 * generating a dedicated keypair — an explicit, accepted v1 trade-off (see SPEC §11) — and states
 * plainly, before submission, that the account needs Docker access on the target machine, which is
 * root-equivalent, mirroring the "deployment guidance, not enforced in code" language SPEC §6
 * already uses for the network-perimeter note.
 */
export const AddHostForm: React.FC<AddHostFormTypes.Props> = (props) => {
  const [values, setValues] = useState<AddHostFormTypes.Values>(INITIAL_VALUES);

  function handleChange<K extends keyof AddHostFormTypes.Values>(
    key: K,
    value: AddHostFormTypes.Values[K],
  ) {
    setValues((prev) => ({ ...prev, [key]: value }));
  }

  function handleSubmit(event: React.FormEvent) {
    event.preventDefault();
    props.onSubmit(values);
  }

  return (
    <form onSubmit={handleSubmit}>
      <Stack gap="sm">
        <Alert color="statusWarn" title="This account needs Docker access">
          The SSH user you provide needs Docker access on the target machine to
          deploy observability tools — that is equivalent to root. Use a
          dedicated key/account where possible.
        </Alert>
        <TextInput
          label="Label"
          placeholder="prod-vps"
          value={values.label}
          onChange={(event) => handleChange("label", event.currentTarget.value)}
        />
        <TextInput
          label="Address"
          placeholder="203.0.113.10"
          required
          value={values.address}
          onChange={(event) =>
            handleChange("address", event.currentTarget.value)
          }
        />
        <NumberInput
          label="SSH port"
          required
          min={1}
          max={65535}
          value={values.sshPort}
          onChange={(value) =>
            handleChange("sshPort", typeof value === "number" ? value : 22)
          }
        />
        <TextInput
          label="SSH user"
          required
          value={values.sshUser}
          onChange={(event) =>
            handleChange("sshUser", event.currentTarget.value)
          }
        />
        <Textarea
          label="SSH private key"
          description="Pasted directly, stored via the same secret_ref mechanism as every other adapter secret (docs/SPEC.md §3)."
          required
          minRows={6}
          autosize
          value={values.sshKey}
          onChange={(event) =>
            handleChange("sshKey", event.currentTarget.value)
          }
        />
        <Group justify="flex-end">
          <Button type="submit" loading={props.isSubmitting}>
            Add host
          </Button>
        </Group>
      </Stack>
    </form>
  );
};
