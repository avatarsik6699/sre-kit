import { Drawer, Stack } from "@mantine/core";
import { useCreateHostMutation } from "~/entities/host";
import { AddHostForm } from "~/features/add-host-form";
import type { AddHostDrawerTypes } from "./add-host-drawer.types";

/** Right-side drawer over the Hosts list (docs/SPEC.md §5.1/§12.1) — mirrors AddSourceDrawer's
 * keep-list-context pattern. */
export const AddHostDrawer: React.FC<AddHostDrawerTypes.Props> = (props) => {
  const createHostMutation = useCreateHostMutation();

  return (
    <Drawer
      opened={props.opened}
      onClose={props.onClose}
      title="Add host"
      position="right"
    >
      <Stack gap="md">
        <AddHostForm
          isSubmitting={createHostMutation.isPending}
          onSubmit={(values) =>
            createHostMutation.mutate(values, { onSuccess: props.onClose })
          }
        />
      </Stack>
    </Drawer>
  );
};
