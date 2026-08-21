import PlatformCard from './PlatformCard.jsx'

export default function DownloadSection({ id, eyebrow, title, desc, builds, accent }) {
  return (
    <section className="section" id={id}>
      <div className="container">
        <div className="section__head reveal">
          <p className="eyebrow">{eyebrow}</p>
          <h2 className="section__title">{title}</h2>
          <p className="section__desc">{desc}</p>
        </div>

        <div className="downloads reveal reveal--delay-1">
          {builds.map((build) => (
            <PlatformCard key={`${build.os}-${build.arch}-${build.format}`} build={build} accent={accent} />
          ))}
        </div>
      </div>
    </section>
  )
}
