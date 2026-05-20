"use client";

const machineProfiles = [
  {
    code: "Model 5X-5 Fine Cleaner System",
    description:
      "Complete grain calibration station containing the main 5X-5 cleaning body, Model 4-72-4.5A Fan (7.5KW), Model 2-0900 Cyclone, integrated inverters electric cabinet, Model W6 anti-broken bucket elevator (1.1KW), and 7PCS white bean spare sifters.",
    specs: ["Capacity: 5 TPH based on wheat density"],
  },
  {
    code: "LCSX Intelligent Photoelectric Color Sorter",
    description:
      "High-resolution optical sorting grid using automated shape recognition algorithms and cloud-connected remote parameter settings via mobile app interfaces.",
    specs: [],
  },
  {
    code: "TQSF Gravity De-Stoner",
    description:
      "Specific gravity heavy impurity separator with double-deck reciprocating screen links and enclosed negative pressure dust extraction.",
    specs: [],
  },
  {
    code: "DCS Electronic Quantitative Packing Scale",
    description:
      "Microcomputer-controlled automated bagging, weighing, and stitching unit.",
    specs: ["Programmable scale range: 10kg - 65kg", "Accuracy: ±0.2%"],
  },
  {
    code: "5XZ Series Gravity Separator",
    description:
      "Premium density separation system designed to extract blighted, insect-damaged, or immature seeds using high-frequency vibration air tables.",
    specs: [],
  },
  {
    code: "5BY Series Automatic Seed Coater",
    description:
      "Automated centrifugal chemical liquid batch mixing system with precise dosing micro-processors for professional seed treatment.",
    specs: [],
  },
  {
    code: "TQLZ Series Vibrating Cleaner",
    description:
      "Heavy-duty industrial pre-cleaning separator powered by dual-vibratory motors for high-volume intake straw and coarse foreign matter removal.",
    specs: [],
  },
  {
    code: "DT Series Heavy-Duty Bucket Elevator",
    description:
      "Vertical bulk material handling conveyor equipped with wear-resistant polymer buckets and mechanical backstop safety brake units.",
    specs: [],
  },
];

export default function HomePage() {
  const scrollToContact = () => {
    const contactSection = document.getElementById("contact");
    contactSection?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  return (
    <div className="page-stack">
      <section className="hero">
        <p className="eyebrow">TradePi Globall</p>
        <h1>Industrial Grain & Seed Processing Request For Quote Portal</h1>
        <p>
          Review certified machine profiles and submit an RFQ directly to our technical sales team.
          This architecture is strictly quote-driven with professional data capture.
        </p>
        <div className="hero-actions">
          <button type="button" className="btn btn-primary" onClick={scrollToContact}>
            Request For Quote
          </button>
        </div>
      </section>

      <section>
        <h2>Factory Machine Profiles</h2>
        <div className="grid">
          {machineProfiles.map((machine) => (
            <article key={machine.code} className="card">
              <h3>{machine.code}</h3>
              <p>{machine.description}</p>
              {machine.specs.length > 0 && (
                <ul>
                  {machine.specs.map((spec) => (
                    <li key={spec}>{spec}</li>
                  ))}
                </ul>
              )}
            </article>
          ))}
        </div>
      </section>

      <section id="contact">
        <h2>Request For Quote</h2>
        <form
          className="card"
          onSubmit={(event) => {
            event.preventDefault();
            alert(
              "Thank you. Your RFQ has been captured by our TradePi Globall technical team. We will respond with a formal configuration proposal.",
            );
          }}
        >
          <label htmlFor="productSelect">Select Machine Model</label>
          <select id="productSelect" name="productSelect" required defaultValue="">
            <option value="" disabled>
              Choose a model for quotation
            </option>
            {machineProfiles.map((machine) => (
              <option key={machine.code} value={machine.code}>
                {machine.code}
              </option>
            ))}
          </select>
          <label htmlFor="company">Company Name</label>
          <input id="company" name="company" type="text" required />
          <label htmlFor="email">Business Email</label>
          <input id="email" name="email" type="email" required />
          <button type="submit" className="btn btn-primary">
            Submit RFQ
          </button>
        </form>
      </section>
    </div>
  );
}
