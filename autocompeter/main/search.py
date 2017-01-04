from elasticsearch_dsl import (
    DocType,
    Float,
    Text,
    Index,
    analyzer,
    Keyword,
    token_filter,
)

from django.conf import settings


edge_ngram_analyzer = analyzer(
    'edge_ngram_analyzer',
    type='custom',
    tokenizer='standard',
    filter=[
        'lowercase',
        token_filter(
            'edge_ngram_filter', type='edgeNGram',
            min_gram=1, max_gram=20
        )
    ]
)


class TitleDoc(DocType):
    id = Keyword()
    domain = Keyword(required=True)
    url = Keyword(required=True, index=False)
    title = Text(
        required=True,
        analyzer=edge_ngram_analyzer,
        search_analyzer='standard'
    )
    popularity = Float()
    group = Keyword()


# create an index and register the doc types
index = Index(settings.ES_INDEX)
index.settings(**settings.ES_INDEX_SETTINGS)
index.doc_type(TitleDoc)
