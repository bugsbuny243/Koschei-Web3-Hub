from fastapi import FastAPI
from pydantic import BaseModel
import os

app = FastAPI()

class Request(BaseModel):
    task: str
    model: str
    input: dict

@app.post('/worker/generate')
def generate(req: Request):
    cloudinary_base = os.getenv('CLOUDINARY_PUBLIC_BASE', 'https://res.cloudinary.com/demo')
    fake_url = f"{cloudinary_base}/koschei/{req.task}/generated-file"
    if req.task == 'stt':
        return {'text': 'transcribed text', 'cloudinary_url': fake_url}
    if req.task in ('tts', 'image', 'image_edit', 'video', 'cinema'):
        return {'cloudinary_url': fake_url, 'provider': 'together', 'model': req.model}
    return {'result': 'unsupported'}
