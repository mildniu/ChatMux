import { ShieldCheck, TerminalSquare } from "lucide-react";
import { GatewayTokenControl } from "./GatewayTokenControl";
import { type GatewayTokenState } from "./useGatewayAccessToken";
import "./gateway-unlock-page.css";

type GatewayUnlockPageProps = {
  error: string;
  tokenState: GatewayTokenState;
};

export function GatewayUnlockPage({ error, tokenState }: GatewayUnlockPageProps) {
  return (
    <main className="gateway-unlock-page">
      <div className="gateway-unlock-bg" aria-hidden="true">
        <span className="gu-aurora" />
        <span className="gu-grid" />
        <span className="gu-beam" />
        <span className="gu-noise" />
      </div>
      <section className="gateway-unlock-panel">
        <div className="gateway-unlock-mark" aria-hidden="true">
          <span className="gu-ring" />
          <TerminalSquare />
        </div>
        <header>
          <strong>
            Chat<em>Mux</em>
          </strong>
          <span>Gateway access required</span>
        </header>
        <GatewayTokenControl tokenState={tokenState} />
        {error ? <p className="gateway-unlock-error">{error}</p> : null}
        <footer className="gateway-unlock-note">
          <ShieldCheck aria-hidden="true" />
          <span>Only connect to a Gateway you deploy and trust.</span>
        </footer>
      </section>
    </main>
  );
}
