const products = [
  {
    name: "5X-5 Fine Cleaner (Amiral Gemisi Komple Sistem)",
    partCode: "Fine Cleaner Model 5X-5",
    description:
      "Capacity: 5 TPH (Tons Per Hour) based on wheat processing density. Total Power Requirement: Combined multi-motor load (6.7KW + 7.5KW + 1.1KW + 1.1KW) running on 380V 50Hz 3-Phase power grids. Physical Dimensions: Main body size 3200 × 1940 × 3600 mm, total net system mass 3250kg + 4000kg auxiliary weights. Interchangeable Sifters: Delivered with 1 suit of 7PCS custom spare sifters engineered specifically for white bean calibration.",
    components: [
      "Main 5X-5 Fine Cleaner Body (13,000 USD component value)",
      "Model 4-72-4.5A Fan (950 USD component value, 7.5KW)",
      "Model 2-0900 Cyclone Air Locker (1,800 USD component value, 1.1KW)",
      "Central Electric Cabinet with Dual Frequency Inverters (1,200 USD component value)",
      "Model W6 Anti-broken Bucket Elevator (1,800 USD component value, 1.1KW)",
      "Custom Maintenance Platform for Elevator & Complete Pipeline Grid",
    ],
  },
  {
    name: "LCSX Intelligent Photoelectric Color Sorter",
    partCode: "LCSX Cloud-Connected Sorting Series",
    description:
      "Sensor Interface: High-resolution digital photoelectric cell detection matrix. Core Systems: Automated real-time sorting execution driven by smart shape recognition algorithms. Connectivity: Integrated remote cloud monitoring architecture allowing mobile phone app parameters calibration. Configuration: Scalable single-channel or dual-channel sorting board slots.",
    components: [],
  },
  {
    name: "TQSF Gravity De-Stoner",
    partCode: "High-Capacity TQSF Series",
    description:
      "Operation Principle: Specific gravity suspension separation utilizing reciprocating double-deck screen links. Structural Advantages: Low power draw profile, enclosed negative pressure frame preventing airborne dust emissions. Adjustability: Independent mechanical layout for screen inclination angle, vibration amplitude, and internal air velocity controls.",
    components: [],
  },
  {
    name: "DCS Electronic Quantitative Packing Scale",
    partCode: "Automated DCS Filling & Stitching Station",
    description:
      "Weight Range: Fully programmable operational scale limits spanning from 10kg up to 65kg bags. Processing Core: High-speed microcomputer sampling processor backing automated weight corrections. Throughput Rate: Production capacity speeds ranging between 420 to 1080 bags per hour at an exact accuracy tolerance of ±0.2%.",
    components: [],
  },
];

export default function ProductsPage() {
  return (
    <div className="page-stack">
      <section>
        <p className="eyebrow">Industrial Product Data</p>
        <h1>Machine Catalog (RFQ Active)</h1>
        <p>
          Official model names, part codes, and technical specification text are listed below.
          Request For Quote (RFO) workflow remains active and excludes price/payment terms in RFQ
          submission.
        </p>
      </section>
      <section className="grid">
        {products.map((product) => (
          <article key={product.name} className="card">
            <h3>{product.name}</h3>
            <p>
              <strong>Exact Part & Model Code:</strong> {product.partCode}
            </p>
            <p>{product.description}</p>
            {product.components.length > 0 && (
              <>
                <p>
                  <strong>System Components Included:</strong>
                </p>
                <ul>
                  {product.components.map((component) => (
                    <li key={component}>{component}</li>
                  ))}
                </ul>
              </>
            )}
            <button className="btn btn-primary">Request For Quote (RFO)</button>
          </article>
        ))}
      </section>
    </div>
  );
}
