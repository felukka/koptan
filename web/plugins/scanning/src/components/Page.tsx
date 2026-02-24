import { Badge, KoptanPage, SampleNote } from '@internal/plugin-koptan-react';

const ROWS = [
  {
    resource: 'payment-cd',
    kind: 'CD',
    findings: [
      ['1 CRITICAL', 'error'],
      ['3 MEDIUM', 'warning'],
    ] as const,
    status: 'At Risk',
  },
  {
    resource: 'auth-cd',
    kind: 'CD',
    findings: [['NONE', 'success']] as const,
    status: 'Secure',
  },
];

export const Page = () => (
  <KoptanPage
    title="The Scan Bay"
    description={
      <>
        Security posture analysis. Hull integrity:{' '}
        <span className="mz-accent">98%</span>
      </>
    }
  >
    <SampleNote>
      Vulnerability scanning is not wired up yet; the operator has no scan data
      source. The rows below only show the planned layout.
    </SampleNote>
    <section className="mz-panel">
      <table className="mz-table">
        <thead>
          <tr>
            <th>Resource</th>
            <th>Kind</th>
            <th>Vulnerabilities</th>
            <th>Status</th>
            <th className="mz-right">Action</th>
          </tr>
        </thead>
        <tbody>
          {ROWS.map((r) => (
            <tr key={r.resource}>
              <td style={{ fontWeight: 700 }}>{r.resource}</td>
              <td className="mz-muted">{r.kind}</td>
              <td>
                {r.findings.map(([label, variant]) => (
                  <span key={label} style={{ marginRight: 8 }}>
                    <Badge variant={variant}>{label}</Badge>
                  </span>
                ))}
              </td>
              <td
                className={
                  r.status === 'At Risk'
                    ? 'mz-status mz-status--bad'
                    : 'mz-status mz-status--ok'
                }
              >
                {r.status}
              </td>
              <td className="mz-right">
                <button type="button" className="mz-link-btn" disabled>
                  View Report
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  </KoptanPage>
);
