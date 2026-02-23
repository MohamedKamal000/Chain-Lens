import React from 'react';
import './TransactionSummary.css';

export default function TransactionSummary({ report }) {
    if (!report) return null;

    const txid = report.txid || '';
    const fee = report.fee !== undefined ? report.fee : '';
    const feerate = report.feerate !== undefined ? report.feerate : '';
    const numInputs = report.vin?.length || 0;
    const numOutputs = report.vout?.length || 0;

    // Map outputs using script_type and value_sats
    const outputs = report.vout?.map((v, idx) => ({
        index: idx,
        type: v.script_type?.toUpperCase() || 'Unknown',  // e.g., "P2WPKH"
        valueBTC: v.value_sats !== undefined ? (v.value_sats / 1e8) : 0
    })) || [];

    return (
        <div className="transaction-summary-card">
            <h3>Transaction Summary</h3>
            <div className="summary-row">
                <span className="label">Transaction ID:</span>
                <span className="value">{txid}</span>
            </div>
            <div className="summary-row">
                <span className="label">Fee:</span>
                <span className="value">{fee} BTC ({feerate} sat/B)</span>
            </div>
            <div className="summary-row">
                <span className="label">Inputs / Outputs:</span>
                <span className="value">{numInputs} / {numOutputs}</span>
            </div>
            <div className="summary-row">
                <span className="label">Outputs:</span>
                <span className="value">
                    {outputs.map(o => (
                        <span key={o.index} className="script-tag">
                            {o.type} – {o.valueBTC} BTC
                        </span>
                    ))}
                </span>
            </div>
        </div>
    );
}