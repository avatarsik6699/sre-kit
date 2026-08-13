import { Stack } from "@mantine/core";
import { Typography } from "~/shared/components/typography";

export type EmptyStateProps = {
  title: string;
  description?: string;
  action?: React.ReactNode;
};

/** Generic "nothing here yet" placeholder — reused wherever a list/collection has zero items. */
export const EmptyState: React.FC<EmptyStateProps> = (props) => {
  return (
    <Stack align="center" justify="center" gap="xs" py="xl">
      <Typography variant="title" order={4}>
        {props.title}
      </Typography>
      {props.description ? (
        <Typography c="dimmed">{props.description}</Typography>
      ) : null}
      {props.action}
    </Stack>
  );
};
