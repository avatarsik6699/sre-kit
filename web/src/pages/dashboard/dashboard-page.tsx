import { Box, SimpleGrid } from "@mantine/core";
import { PageContainer } from "~/shared/components/page-container";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import { useSourcesQuery } from "~/entities/source";
import { StatusTile } from "~/widgets/status-tile";

/** Route-level composition for / — dense grid of live status tiles (docs/SPEC.md §5.1/§5.3). */
export const DashboardPage: React.FC = () => {
  const sourcesQuery = useSourcesQuery();

  return (
    <PageContainer>
      <Box mb="md">
        <Typography variant="title" order={2}>
          Dashboard
        </Typography>
      </Box>
      {sourcesQuery.data && sourcesQuery.data.length === 0 ? (
        <EmptyState
          title="No sources yet"
          description="Add a source from the Sources page to see it here."
        />
      ) : (
        <SimpleGrid cols={{ base: 1, sm: 2, lg: 3, xl: 4 }} spacing="md">
          {(sourcesQuery.data ?? []).map((source) => (
            <StatusTile key={source.id} source={source} />
          ))}
        </SimpleGrid>
      )}
    </PageContainer>
  );
};
