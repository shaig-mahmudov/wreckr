import { notFound } from "next/navigation";
import { ScenarioRoute } from "../../../../../components/scenario-route";

type ScenarioVersionPageProps = {
  params: Promise<{
    id: string;
    version: string;
  }>;
};

export default async function ScenarioVersionPage({ params }: ScenarioVersionPageProps) {
  const { id, version } = await params;
  const versionNumber = Number.parseInt(version, 10);
  if (!Number.isInteger(versionNumber) || versionNumber < 1) {
    notFound();
  }
  return <ScenarioRoute scenarioID={id} versionNumber={versionNumber} />;
}
