import { BaseAgent } from "./base-agent";
import type { AgentState } from "../types";
import { buildAutoYieldGraph } from "../graph/auto-yield-optimizer";

export class AutoYieldOptimizerAgent extends BaseAgent {
  private app = buildAutoYieldGraph();

  constructor() {
    super("AutoYieldOptimizerAgent");
  }

  async run(input: AgentState): Promise<AgentState> {
    this.withLog("start", input.prompt);
    const result = await this.app.invoke(input);
    this.withLog("done", result.summary ?? "no summary");
    return result;
  }
}
