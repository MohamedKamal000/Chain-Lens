import React from 'react';
import './TransactionReportTable.css';

export default function TransactionReportTable({ report }) {
    if (!report) return null;

    // Map transaction fields to human-friendly explanations
    const sectionDescriptions = {
        txid: 'Unique identifier of the transaction.',
        version: 'Protocol version used for this transaction.',
        locktime: 'Earliest time or block this transaction can be confirmed.',
        vin: 'Inputs: the coins or UTXOs being spent in this transaction.',
        vout: 'Outputs: where the coins are sent and in what amount.',
        fee: 'Transaction fee paid to the miners for processing.',
        size: 'Size of the transaction in bytes.',
        weight: 'Weight of the transaction used for block inclusion priority.',
        // Add more fields here if needed
    };

    return (
        <div className="transaction-table-card">
            <h3>Transaction Explained</h3>
            <table className="transaction-table">
                <thead>
                <tr>
                    <th>Section</th>
                    <th>Explanation</th>
                </tr>
                </thead>
                <tbody>
                {Object.keys(sectionDescriptions).map(key => (
                    <tr key={key}>
                        <td>{key}</td>
                        <td>{sectionDescriptions[key]}</td>
                    </tr>
                ))}
                </tbody>
            </table>
        </div>
    );
}