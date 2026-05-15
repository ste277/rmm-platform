import { useEffect, useState } from "react";
import { ComplianceReport, getComplianceReports } from "../lib/api";

export function CompliancePage() {
  const [reports, setReports] = useState<ComplianceReport[]>([]);

  useEffect(() => {
    getComplianceReports().then(setReports).catch(console.error);
  }, []);

  return (
    <main>
      <h1>Compliance</h1>
      <div className="card-list">
        {reports.map((report) => (
          <article className="card" key={report.agent_id}>
            <h2>{report.agent_id}</h2>
            <p>Status: {report.status}</p>
            {report.findings.map((finding) => (
              <p key={`${finding.category}-${finding.resource_id}`}>
                {finding.category}: {finding.resource_id} [{finding.status}]
              </p>
            ))}
          </article>
        ))}
      </div>
    </main>
  );
}
