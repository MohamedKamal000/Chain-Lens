// src/TransactionGraphRadial.jsx
import React from "react";

export default function TransactionGraphRadial({ report }) {
    if (!report || !report.ok) return null;

    const { vin, vout, fee_sats } = report;
    const width = 700;
    const height = 400;
    const marginX = 150; // distance from edges to input/output circles
    const paddingY = 50;

    const inputSpacing = (height - 2 * paddingY) / (vin.length - 1 || 1);
    const outputSpacing = (height - 2 * paddingY) / (vout.length - 1 || 1);

    const centerX = width / 2;
    const centerY = height / 2;

    const scaleRadius = (value, maxValue) => 10 + (value / maxValue) * 15;
    const maxInput = Math.max(...vin.map(i => i.prevout.value_sats));
    const maxOutput = Math.max(...vout.map(o => o.value_sats));

    return (
        <div className="card graph-view">
            <h3>Transaction Flow</h3>
            <svg width="100%" height={height}>
                {/* Inputs */}
                {vin.map((i, idx) => {
                    const cy = paddingY + idx * inputSpacing;
                    const r = scaleRadius(i.prevout.value_sats, maxInput);
                    return (
                        <g key={`in-${idx}`}>
                            <circle cx={centerX - marginX} cy={cy} r={r} fill="#4fc3f7" />
                            <text
                                x={centerX - marginX}
                                y={cy}
                                textAnchor="middle"
                                dy=".35em"
                                fill="#fff"
                                fontSize="10"
                                fontWeight="600"
                            >
                                {i.prevout.value_sats}
                            </text>
                        </g>
                    );
                })}

                {/* Outputs */}
                {vout.map((o, idx) => {
                    const cy = paddingY + idx * outputSpacing;
                    const r = scaleRadius(o.value_sats, maxOutput);
                    return (
                        <g key={`out-${idx}`}>
                            <circle cx={centerX + marginX} cy={cy} r={r} fill="#81c784" />
                            <text
                                x={centerX + marginX}
                                y={cy}
                                textAnchor="middle"
                                dy=".35em"
                                fill="#fff"
                                fontSize="10"
                                fontWeight="600"
                            >
                                {o.value_sats}
                            </text>
                        </g>
                    );
                })}

                {/* Fee */}
                {fee_sats > 0 && (
                    <g>
                        <circle cx={centerX} cy={centerY} r={20} fill="#ffb74d" />
                        <text
                            x={centerX}
                            y={centerY}
                            textAnchor="middle"
                            dy=".35em"
                            fill="#333"
                            fontSize="12"
                            fontWeight="700"
                        >
                            {fee_sats}
                        </text>
                    </g>
                )}

                {/* Arcs */}
                {vin.map((i, iidx) =>
                    vout.map((o, oidx) => {
                        const startX = centerX - marginX;
                        const startY = paddingY + iidx * inputSpacing;
                        const endX = centerX + marginX;
                        const endY = paddingY + oidx * outputSpacing;
                        const midX = centerX;
                        return (
                            <path
                                key={`arc-${iidx}-${oidx}`}
                                d={`M ${startX} ${startY} C ${midX} ${startY}, ${midX} ${endY}, ${endX} ${endY}`}
                                stroke="#888"
                                strokeWidth="2"
                                fill="none"
                                strokeOpacity="0.6"
                            />
                        );
                    })
                )}
            </svg>

            <div className="legend">
                <span style={{ color: "#4fc3f7" }}>● Input</span>
                <span style={{ color: "#81c784", marginLeft: "1em" }}>● Output</span>
                <span style={{ color: "#ffb74d", marginLeft: "1em" }}>● Fee</span>
            </div>
        </div>
    );
}