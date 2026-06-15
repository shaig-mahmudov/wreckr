import { ScenarioRoute } from "../../../components/scenario-route";

type ScenarioPageProps = {
  params: Promise<{
    id: string;
  }>;
};

export default async function ScenarioPage({ params }: ScenarioPageProps) {
  const { id } = await params;
  return <ScenarioRoute scenarioID={id} />;
}
