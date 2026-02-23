// src/TransactionStorySami.jsx
import React from "react";
import { Tooltip } from "react-tooltip";
import "react-tooltip/dist/react-tooltip.css";

export default function TransactionStoryNarrative({ report }) {
    if (!report || !report.ok) return null;

    const {
        fee_sats,
        fee_rate_sat_vb,
        vin,
        vout,
        segwit,
        rbf_signaling,
        locktime_type,
        locktime_value,
        segwit_savings
    } = report;

    const totalInput = vin.reduce((sum, i) => sum + (i.prevout?.value_sats || 0), 0);
    const inputLabels = vin.map((i, idx) => `Sami Wallet ${idx + 1}`);
    const outputLabels = vout.map((o, idx) =>
        idx === 0 ? "Café Wallet" : "Sami Remaining/LeftOver Sats"
    );

    const tipText = (id, text) => (
        <Tooltip anchorId={id} content={text} place="top" />
    );

    return (
        <div className="card story-view">
            <h3>Transaction Story</h3>

            <div className="story-section narrative">
                <p>
                    Sami is paying the Café Shop. Total Spent is{" "}
                    <span className="amount">{totalInput} sats</span>.
                </p>
            </div>

            <div className="story-section">
                <h4>Inputs</h4>
                <ul>
                    {vin.map((i, idx) => (
                        <li key={idx}>
                            {inputLabels[idx]}: <span className="address">{i.address || "unknown"}</span>, paid{" "}
                            <span className="amount">{i.prevout?.value_sats || 0}</span> sats
                        </li>
                    ))}
                </ul>
            </div>

            <div className="story-section">
                <h4>Outputs</h4>
                <ul>
                    {vout.map((o, idx) => (
                        <li key={idx}>
                            {outputLabels[idx]}: <span className="address">{o.address || "unknown"}</span> received{" "}
                            <span className="amount">{o.value_sats}</span> sats
                        </li>
                    ))}
                </ul>
            </div>

            <div className="story-section">
                <b>Fee:</b> <span className="amount">{fee_sats}</span> sats ({fee_rate_sat_vb.toFixed(2)} sat/vbyte){" "}
                <span id="fee-tip" className="tooltip-icon">?</span>
                {tipText("fee-tip", "This is the amount paid to miners to include your transaction quickly.")}
            </div>

            <div className="story-section">
                <b>SegWit:</b> {segwit ? "Yes" : "No"}{" "}
                <span id="segwit-tip" className="tooltip-icon">?</span>
                {tipText("segwit-tip", "SegWit reduces transaction size so fees are lower.")}
            </div>

            <div className="story-section">
                <b>RBF:</b> {rbf_signaling ? "Yes" : "No"}{" "}
                <span id="rbf-tip" className="tooltip-icon">?</span>
                {tipText("rbf-tip", "RBF allows replacing this transaction with a higher fee if needed.")}
            </div>

            <div className="story-section">
                <b>Locktime:</b> {locktime_type} ({locktime_value}){" "}
                <span id="locktime-tip" className="tooltip-icon">?</span>
                {tipText("locktime-tip", "Locktime restricts when this transaction can be added to the blockchain.")}
            </div>

            {segwit && segwit_savings && (
                <div className="story-section">
                    <b>SegWit Savings:</b> <span className="amount">{segwit_savings.savings_pct}%</span> smaller{" "}
                    <span id="segwit-savings-tip" className="tooltip-icon">?</span>
                    {tipText("segwit-savings-tip", "SegWit reduces transaction size and therefore the fee.")}
                </div>
            )}
        </div>
    );
}