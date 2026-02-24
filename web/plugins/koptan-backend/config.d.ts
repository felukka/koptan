export interface Config {
  koptan?: {
    /** Only read and create Koptan resources in this namespace. */
    namespace?: string;
    /** The kubeconfig context to use instead of the current one. */
    kubeContext?: string;
    /**
     * Derives each SelfService agent's API token; must equal the operator's
     * KOPTAN_AGENT_KEY. Without it the Self Service page cannot send prompts.
     * @visibility secret
     */
    agentKey?: string;
  };
}
