import { ScenarioEditor } from "../../../components/scenario-editor";
import { checkoutRaceScenario } from "../../../lib/sample-scenarios";

export default function NewScenarioPage() {
  return (
    <ScenarioEditor
      eyebrow="New Scenario"
      initialScenario={checkoutRaceScenario}
      saveLabel="Create"
      title="Create Scenario"
    />
  );
}
