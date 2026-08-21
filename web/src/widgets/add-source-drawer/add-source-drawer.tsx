import { useState } from "react";
import { Drawer, Select, Stack } from "~/shared/ui";
import { useAdaptersQuery } from "~/entities/adapter";
import { useCreateSourceMutation } from "~/entities/source";
import { useProjectsQuery } from "~/entities/project";
import { AddSourceForm } from "~/features/add-source-form";
import { Typography } from "~/shared/components/typography";
import type { AddSourceDrawerTypes } from "./add-source-drawer.types";

/** Right-side drawer over the Sources list (docs/SPEC.md §5.1/§5.3) — keeps list context while
 * adding a source. */
export const AddSourceDrawer: React.FC<AddSourceDrawerTypes.Props> = (
  props,
) => {
  const adaptersQuery = useAdaptersQuery();
  const createSourceMutation = useCreateSourceMutation();
  const projectsQuery = useProjectsQuery();
  const [selectedProjectId, setSelectedProjectId] = useState<string | null>(
    "default",
  );
  const [selectedAdapterName, setSelectedAdapterName] = useState<string | null>(
    null,
  );

  function handleClose() {
    setSelectedAdapterName(null);
    props.onClose();
  }

  const selectedAdapter = adaptersQuery.data?.find(
    (adapter) => adapter.name === selectedAdapterName,
  );

  return (
    <Drawer
      opened={props.opened}
      onClose={handleClose}
      title="Add source"
      position="right"
    >
      <Stack gap="md">
        <Select
          label="Project"
          data={(projectsQuery.data ?? []).map((project) => ({
            value: project.id,
            label: project.name,
          }))}
          value={selectedProjectId}
          onChange={setSelectedProjectId}
        />
        <Select
          label="Adapter"
          placeholder="Choose an adapter"
          data={(adaptersQuery.data ?? []).map((adapter) => ({
            value: adapter.name,
            label: `${adapter.name} (${adapter.mode})`,
          }))}
          value={selectedAdapterName}
          onChange={setSelectedAdapterName}
        />
        {adaptersQuery.data && adaptersQuery.data.length === 0 ? (
          <Typography c="dimmed">No adapters installed.</Typography>
        ) : null}
        {selectedAdapter ? (
          <AddSourceForm
            adapter={selectedAdapter}
            isSubmitting={createSourceMutation.isPending}
            onSubmit={(config) =>
              createSourceMutation.mutate(
                {
                  projectId: selectedProjectId ?? "default",
                  adapterId: selectedAdapter.name,
                  config,
                },
                { onSuccess: handleClose },
              )
            }
          />
        ) : null}
      </Stack>
    </Drawer>
  );
};
