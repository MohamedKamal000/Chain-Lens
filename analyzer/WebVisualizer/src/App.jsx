import { useState } from 'react';
import './App.css';
import { Tooltip } from 'react-tooltip';
import TransactionGraphRadial from "./TransactionGraph.jsx";
import TransactionStoryNarrative from "./TransactionStory.jsx";
import TransactionReportTable from "./TransactionReportTable.jsx";
import TransactionSummary from "./TransactionSummary.jsx";

function App() {
    const [mode, setMode] = useState('transaction');
    const [txTab, setTxTab] = useState('json');
    const [fixture, setFixture] = useState('');
    const [rawTx, setRawTx] = useState('');
    const [prevouts, setPrevouts] = useState('');
    const [analyzeResult, setAnalyzeResult] = useState(null);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState(null);

    const handleAnalyze = async () => {
        setLoading(true);
        setError(null);
        setAnalyzeResult(null);

        let body;
        if (txTab === 'json') {
            try { body = JSON.parse(fixture); }
            catch { setError('Invalid JSON.'); setLoading(false); return; }
        } else {
            if (!rawTx.trim()) { setError('Raw transaction required.'); setLoading(false); return; }
            let prevArr = [];
            if (prevouts.trim()) {
                try {
                    prevArr = JSON.parse(prevouts);
                    if (!Array.isArray(prevArr)) throw new Error();
                }
                catch {
                    setError('Prevouts must be JSON array.');
                    setLoading(false);
                    return;
                }
            }
            body = { network: 'mainnet', raw_tx: rawTx.trim(), prevouts: prevArr };
        }

        body.mode = 'transaction';

        try {
            const res = await fetch('http://localhost:8080/api/analyze', {
                method: 'POST',
                headers: {'Content-Type':'application/json'},
                body: JSON.stringify(body)
            });
            const data = await res.json();
            setAnalyzeResult(data);
            if (!data.ok) setError(data.error?.message || 'Analysis failed.');
        } catch (err) {
            console.error(err);
            setError('Failed to analyze transaction');
        }

        setLoading(false);
    };

    const tip = (text, id) => (
        <>
            <span
                data-tooltip-id={id}
                data-tooltip-content={text}
                className="tooltip-icon"
            >
                ?
            </span>
            <Tooltip id={id} />
        </>
    );

    return (
        <div className="container">
            <header>
                <h1>Bitcoin Transaction Visualizer</h1>
                <div className="mode-switch">
                    <button onClick={()=>setMode('transaction')} className={mode==='transaction'?'active':''}>Transaction</button>
                </div>
            </header>

            {mode==='transaction' && (
                <section>
                    <div className="tx-tabs">
                        <button className={txTab==='json'?'active':''} onClick={()=>setTxTab('json')}>Paste JSON Fixture</button>
                        <button className={txTab==='raw'?'active':''} onClick={()=>setTxTab('raw')}>Raw TX + Prevouts</button>
                    </div>

                    {txTab==='json' &&
                        <textarea
                            rows={8}
                            value={fixture}
                            onChange={e=>setFixture(e.target.value)}
                            placeholder="Paste fixture JSON here"
                        />
                    }

                    {txTab==='raw' && <>
                        <input
                            type="text"
                            value={rawTx}
                            onChange={e=>setRawTx(e.target.value)}
                            placeholder="Raw transaction hex"
                        />
                        <textarea
                            rows={4}
                            value={prevouts}
                            onChange={e=>setPrevouts(e.target.value)}
                            placeholder="Prevouts JSON array (optional)"
                        />
                    </>}

                    <button onClick={handleAnalyze} disabled={loading}>
                        {loading ? 'Analyzing...' : 'Analyze Transaction'}
                    </button>

                    {error && <div className="error">{error}</div>}

                    {analyzeResult?.ok && (
                        <>
                            <TransactionStoryNarrative report={analyzeResult.data || analyzeResult} tip={tip} />
                            <TransactionGraphRadial report={analyzeResult.data || analyzeResult} />
                            <TransactionSummary report={analyzeResult.data || analyzeResult} />
                            <TransactionReportTable report={analyzeResult.data || analyzeResult} />
                        </>
                    )}
                </section>
            )}
        </div>
    );
}


export default App;