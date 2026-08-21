import { Box, SimpleGrid, Stack } from "~/shared/ui";
import { PageContainer } from "~/shared/components/page-container";
import { Typography } from "~/shared/components/typography";
import { EmptyState } from "~/shared/components/empty-state";
import { useSourcesQuery, type Source } from "~/entities/source";
import { useProjectsQuery, type Project } from "~/entities/project";
import {
  useAdaptersQuery,
  type AdapterManifest,
} from "~/entities/adapter";
import { StatusTile } from "~/widgets/status-tile";
import { RecentAlertsRail } from "~/widgets/recent-alerts-rail";
import { TelemetrySummary } from "~/widgets/telemetry-summary";

/** Route-level composition for / — dense grid of live status tiles (docs/SPEC.md §5.1/§5.3). */
export const DashboardPage: React.FC = () => {
  const sourcesQuery = useSourcesQuery();
  const projectsQuery = useProjectsQuery();
  const adaptersQuery = useAdaptersQuery();

  return (
    <PageContainer>
      <Box mb="md">
        <Typography variant="title" order={2}>
          Dashboard
        </Typography>
      </Box>
      <Box mb="md">
        <RecentAlertsRail />
      </Box>
      {sourcesQuery.data && sourcesQuery.data.length === 0 ? (
        <EmptyState
          title="No sources yet"
          description="Add a source from the Sources page to see it here."
        />
      ) : (
        <Stack gap="xl">
          {(
            projectsQuery.data ?? [
              {
                id: "default",
                name: "Default",
                slug: "default",
                description: "",
              },
            ]
          ).map((project: Project) => {
            const sources = (sourcesQuery.data ?? []).filter(
              (source: Source) =>
                (source.projectId ?? "default") === project.id,
            );
            if (sources.length === 0) return null;
            return (
              <section key={project.id}>
                <Typography variant="title" order={2}>
                  {project.name}
                </Typography>
                {project.description ? (
                  <Typography c="dimmed">{project.description}</Typography>
                ) : null}
                <SimpleGrid spacing="md">
                  {sources.map((source: Source) => (
                    <Stack key={source.id} gap="sm">
                      <StatusTile source={source} />
                      <TelemetrySummary
                        sourceId={source.id}
                        title="Current measurements"
                        presentationSchema={
                          adaptersQuery.data?.find(
                            (adapter: AdapterManifest) =>
                              adapter.name === source.adapterId,
                          )?.presentationSchema
                        }
                      />
                    </Stack>
                  ))}
                </SimpleGrid>
              </section>
            );
          })}
        </Stack>
      )}
    </PageContainer>
  );
};
