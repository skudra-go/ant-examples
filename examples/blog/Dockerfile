# Built from the folder holding the ant and ant-examples checkouts, because the
# go.mod replaces ant with a local copy. Once ant is tagged, drop the replace and
# the first COPY, and build from this folder.
FROM golang:1.27-alpine AS build
WORKDIR /src
COPY ant ./ant
COPY ant-examples/examples/blog ./ant-examples/examples/blog
WORKDIR /src/ant-examples/examples/blog
RUN CGO_ENABLED=0 go build -trimpath -o /out/server ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -o /out/migrate ./cmd/migrate

# Both binaries embed the migrations, so the image holds nothing else.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/ /app/
EXPOSE 8080
ENTRYPOINT []
CMD ["/app/server"]
